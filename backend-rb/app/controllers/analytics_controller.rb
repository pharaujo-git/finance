# frozen_string_literal: true

# The query field keys here are lowercase (months, year, from, to), unlike the
# transaction search's PascalCase ones. That difference is inherited from the
# .NET action signatures and the parity tests assert it.
class AnalyticsController < ApplicationController
  def summary = render_api(service.summary(caller_id, Time.now.utc))

  def networth
    reader = Api::QueryReader.new(request.query_parameters)
    months = reader.number_or("months", "months", Services::AnalyticsService::DEFAULT_NET_WORTH_MONTHS)
    reader.done
    render_api(service.net_worth(caller_id, Time.now.utc, months))
  end

  def cashflow
    reader = Api::QueryReader.new(request.query_parameters)
    months = reader.number_or("months", "months", Services::AnalyticsService::DEFAULT_CASHFLOW_MONTHS)
    reader.done
    render_api(service.cashflow(caller_id, Time.now.utc, months))
  end

  def spending
    month = Api::QueryReader.new(request.query_parameters).text("month")
    render_api(service.spending(caller_id, Time.now.utc, month))
  end

  def monthly_report
    reader = Api::QueryReader.new(request.query_parameters)
    year = reader.number_or("year", "year", Time.now.utc.year)
    reader.done
    render_api(service.monthly_report(caller_id, year))
  end

  def category_report
    reader = Api::QueryReader.new(request.query_parameters)
    date_from = reader.moment("from", "from")
    date_to = reader.moment("to", "to")
    reader.done
    render_api(service.category_report(caller_id, date_from, date_to))
  end

  private

  def service
    Services::AnalyticsService.new(
      Repositories::TransactionRepository.new(db),
      Repositories::AccountRepository.new(db),
      Services::CategoryService.new(Repositories::CategoryRepository.new(db))
    )
  end
end
