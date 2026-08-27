# frozen_string_literal: true

require "simplecov"
require "simplecov_json_formatter"

SimpleCov.start do
  # The scanner reads the JSON formatter's output; the HTML one is noise in CI.
  formatter SimpleCov::Formatter::JSONFormatter
  skip "/spec/"
  skip "/config/"
  skip "/vendor/"
  # `cover` also counts files that no example loaded, so a wholly untested
  # file shows as 0% rather than vanishing from the report.
  cover "{app,lib}/**/*.rb"
end

RSpec.configure do |config|
  config.expect_with(:rspec) { |expectations| expectations.syntax = :expect }
  config.disable_monkey_patching!
  config.order = :defined
end
