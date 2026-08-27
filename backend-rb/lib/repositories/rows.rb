# frozen_string_literal: true

module Repositories
  # Turns a pg row -- every column arrives as text -- into domain values.
  module Rows
    module_function

    def money(value)
      return Domain::MONEY_ZERO if value.nil?

      Domain::Money.parse(value) || Domain::MONEY_ZERO
    end

    def instant(value) = Domain::Instant.from_pg(value)
    def instant_or_nil(value) = value.nil? ? nil : Domain::Instant.from_pg(value)
    def enum(kind, value) = Domain::Enums.of(kind, value.to_i)
    def flag(value) = value == "t" || value == true
    def tags(value) = Domain::Tags.split(value)
  end
end
