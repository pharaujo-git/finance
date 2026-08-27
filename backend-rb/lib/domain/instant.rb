# frozen_string_literal: true

require "time"

module Domain
  # A moment in time that keeps the precision Postgres stores.
  #
  # The column is `timestamp with time zone`, which holds microseconds, and the
  # other four backends put all six digits on the wire. Ruby's Time can hold
  # them, but only if the value is never routed through a second-precision
  # format, so the parsed Time is kept as-is and rendered directly.
  class Instant
    include Comparable

    attr_reader :time

    def initialize(time)
      @time = time.utc
      freeze
    end

    class << self
      def now = new(Time.now)
      def from_time(value) = new(value)

      # Reads the text form pg produces, keeping every fractional digit.
      def from_pg(raw)
        return new(raw) if raw.is_a?(Time)
        return nil if raw.nil?

        new(Time.parse(raw.to_s))
      rescue ArgumentError
        nil
      end

      # Accepts the layouts the model binder does, assuming UTC when no zone is
      # given: RFC3339, second- and minute-precision local timestamps, and the
      # bare date the frontend's <input type="date"> posts.
      def parse_wire(raw)
        text = raw.to_s.strip
        return nil if text.empty?

        candidate =
          case text
          when /\A\d{4}-\d{2}-\d{2}\z/ then "#{text}T00:00:00Z"
          when /\A\d{4}-\d{2}-\d{2}T\d{2}:\d{2}\z/ then "#{text}:00Z"
          when /\A\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?\z/ then "#{text}Z"
          when /\A\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})\z/ then text
          else nil # no layout matched, so there is nothing to hand to Time.parse
          end
        return nil if candidate.nil?

        new(Time.parse(candidate))
      rescue ArgumentError
        nil
      end
    end

    def <=>(other) = time <=> other.time

    # RFC3339 in UTC, with trailing zeros trimmed off the fraction -- which is
    # how Go marshals a time and therefore what the other backends emit.
    def to_s
      base = time.strftime("%Y-%m-%dT%H:%M:%S")
      fraction = time.nsec.zero? ? "" : format("%09d", time.nsec).sub(/0+\z/, "")
      fraction.empty? ? "#{base}Z" : "#{base}.#{fraction}Z"
    end

    def as_json(*) = to_s
    def to_param = time
    def inspect = "#<Instant #{self}>"
  end
end
