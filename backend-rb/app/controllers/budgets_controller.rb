# frozen_string_literal: true

class BudgetsController < ApplicationController
  def index
    # An absent key means "this month"; an empty one is a validation failure.
    month = Api::QueryReader.new(request.query_parameters).text("month")
    render_api(service.list_all(caller_id, month))
  end

  def create
    body = Api::Schemas.create_budget(body_params)
    render_api(service.create(caller_id, body[:category_id], body[:month], body[:limit]))
  end

  def update
    body = Api::Schemas.update_budget(body_params)
    render_api(service.update(caller_id, path_id, body[:limit]))
  end

  def destroy
    service.remove(caller_id, path_id)
    render_no_content
  end

  private

  def service
    Services::BudgetService.new(
      Repositories::BudgetRepository.new(db),
      Repositories::TransactionRepository.new(db),
      Services::CategoryService.new(Repositories::CategoryRepository.new(db))
    )
  end
end
