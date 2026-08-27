# frozen_string_literal: true

# The request specs drive the real Rails stack against a throwaway Postgres
# schema, built from the same dbmate migrations that ship. Without
# TEST_DATABASE_URL they are skipped, so `rspec` stays green with no Docker.
ENV["RAILS_ENV"] ||= "test"
ENV["SECRET_KEY_BASE"] ||= "test-secret-key-base"
# The boot-time container needs a DATABASE_URL; the harness replaces it with one
# pinned to its own schema.
ENV["DATABASE_URL"] ||= ENV["TEST_DATABASE_URL"] || "postgresql://localhost/postgres"

require_relative "../config/environment"
require "rspec/rails"
require "securerandom"

module Harness
  DEMO_PASSWORD = "Passw0rd!123"

  module_function

  def database_url
    url = ENV["TEST_DATABASE_URL"].to_s.strip
    url.empty? ? nil : url
  end

  def available? = !database_url.nil?

  # The `-- migrate:up` half of a dbmate migration.
  def migration_up(name)
    path = Rails.root.join("..", "db", "migrations", name)
    File.read(path).split(/^--\s*migrate:down\s*$/).first.sub(/^--\s*migrate:up\s*$/, "")
  end

  # Opens a pool pinned to a schema of its own and installs it in the container.
  def install!
    schema = "rspec_#{SecureRandom.hex(10)}"

    admin = PG.connect(database_url)
    admin.exec(%(CREATE SCHEMA "#{schema}"))
    admin.close

    db = Core::Database.new(database_url, schema: schema)
    %w[0001_baseline.sql 0002_seed_default_categories.sql].each do |name|
      db.exec_script(migration_up(name))
    end

    settings = Finance::Settings.new(
      database_url: database_url, jwt_secret: Finance::LOCAL_DEVELOPMENT_SECRET,
      port: 8084, allowed_origins: [ "http://localhost:5173" ]
    )
    Rails.configuration.x.container = Finance::Container.new(settings, db: db)
    schema
  end

  def uninstall!(schema)
    Rails.configuration.x.container&.db&.close
    Rails.configuration.x.container = nil

    cleanup = PG.connect(database_url)
    cleanup.exec(%(DROP SCHEMA "#{schema}" CASCADE))
    cleanup.close
  end
end

RSpec.configure do |config|
  config.infer_spec_type_from_file_location!

  config.around(:each, type: :request) do |example|
    if Harness.available?
      schema = Harness.install!
      begin
        example.run
      ensure
        Harness.uninstall!(schema)
      end
    else
      skip "TEST_DATABASE_URL is not set; skipping the Postgres request specs"
    end
  end
end
