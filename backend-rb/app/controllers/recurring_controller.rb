# frozen_string_literal: true

class RecurringController < ApplicationController
  def index = render_api(service.list_all(caller_id))
  def create = render_api(service.create(caller_id, Api::Schemas.recurring(body_params)))

  def update
    render_api(service.update(caller_id, path_id, Api::Schemas.recurring(body_params)))
  end

  def destroy
    service.remove(caller_id, path_id)
    render_no_content
  end

  private

  def service
    Services::RecurringService.new(
      Repositories::RecurringRepository.new(db),
      Repositories::TransactionRepository.new(db),
      Repositories::AccountRepository.new(db),
      Services::CategoryService.new(Repositories::CategoryRepository.new(db))
    )
  end
end
