# frozen_string_literal: true

require "date"

module Domain
  # Calendar arithmetic that matches .NET's DateTime, and the YYYY-MM key.
  module Dates
    MONTH_FORMAT_MESSAGE = "Month must be in YYYY-MM format."
    YEAR_RANGE_MESSAGE = "Year must be between 1900 and 9999."

    MIN_REPORT_YEAR = 1900
    MAX_REPORT_YEAR = 9999
    MIN_WINDOW_MONTH = 1
    MAX_WINDOW_MONTH = 120

    module_function

    # .NET's DateTime.AddMonths: clamps the day rather than rolling over, so
    # 31 Jan + 1 month is 28 Feb -- and adding another gives 28 Mar, not 31 Mar.
    # Clock time is preserved.
    def add_months(moment, months)
      total = (moment.month - 1) + months
      year = moment.year + total.fdiv(12).floor
      month = (total % 12) + 1
      day = [ moment.day, ::Date.new(year, month, -1).day ].min

      Time.utc(year, month, day, moment.hour, moment.min, moment.sec, moment.nsec / 1000.0)
    end

    # Also clamping, so 29 Feb becomes 28 Feb in a common year.
    def add_years(moment, years) = add_months(moment, years * 12)

    def start_of_month(moment) = Time.utc(moment.year, moment.month, 1)

    def first_day_utc(year, month) = Time.utc(year, month, 1)

    def month_key(moment) = moment.utc.strftime("%Y-%m")

    # Midnight UTC on day 1 of that month, or nil when it is not a real month.
    def try_parse_month(value)
      text = value.to_s.strip
      return nil unless /\A\d{4}-\d{2}\z/.match?(text)

      year = text[0, 4].to_i
      month = text[5, 2].to_i
      return nil if month < 1 || month > 12

      Time.utc(year, month, 1)
    end

    # `count` month starts ending with the reference's own month, oldest first.
    def trailing_months(reference, count)
      anchor = start_of_month(reference)
      (0...count).map { |offset| add_months(anchor, offset - count + 1) }
    end

    def clamp_months(months) = months.clamp(MIN_WINDOW_MONTH, MAX_WINDOW_MONTH)
  end
end
