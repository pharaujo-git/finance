# frozen_string_literal: true

require "pg"

module Core
  # A small Postgres connection pool.
  #
  # The schema is owned by the backend-neutral dbmate migrations in
  # db/migrations, so nothing here issues DDL. Table and column names are EF
  # Core's quoted PascalCase, which is why every query quotes its identifiers.
  class Database
    DEFAULT_SIZE = 5

    def initialize(url, size: DEFAULT_SIZE, schema: nil)
      @url = url
      @schema = schema
      @available = Queue.new
      @created = 0
      @size = size
      @mutex = Mutex.new
    end

    # Borrows a connection for one unit of work and always returns it.
    def with_connection
      connection = checkout
      begin
        yield connection
      ensure
        @available << connection
      end
    end

    def exec(sql, params = [])
      with_connection { |connection| connection.exec_params(sql, params) }
    end

    # Runs a multi-statement script over the simple protocol. exec_params
    # prepares the statement, and Postgres refuses more than one command in a
    # prepared statement -- which is what a migration file is. For migrations
    # only: it takes no parameters, so nothing user-supplied reaches it.
    def exec_script(sql)
      with_connection { |connection| connection.exec(sql) }
    end

    # Runs the block inside one transaction on a single connection.
    def transaction
      with_connection do |connection|
        connection.exec("BEGIN")
        begin
          result = yield connection
          connection.exec("COMMIT")
          result
        rescue StandardError
          connection.exec("ROLLBACK")
          raise
        end
      end
    end

    def close
      until @available.empty?
        connection = (@available.pop(true) rescue break)
        connection.close
      end
    end

    private

    def checkout
      @mutex.synchronize do
        if @available.empty? && @created < @size
          @created += 1
          return connect
        end
      end
      @available.pop
    end

    def connect
      connection = PG.connect(@url)
      # A per-connection search_path is how the test harness pins the pool to a
      # throwaway schema without touching the connection string.
      connection.exec("SET search_path TO #{PG::Connection.quote_ident(@schema)}") if @schema
      connection
    end
  end
end
