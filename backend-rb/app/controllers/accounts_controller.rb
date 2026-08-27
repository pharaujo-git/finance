# frozen_string_literal: true

class AccountsController < ApplicationController
  def index = render_api(service.list_all(caller_id))
  def show = render_api(service.get(caller_id, path_id))

  def create = render_api(service.create(caller_id, Api::Schemas.account(body_params)))

  def update
    render_api(service.update(caller_id, path_id, Api::Schemas.account(body_params)))
  end

  def destroy
    service.archive(caller_id, path_id)
    render_no_content
  end

  private

  def service
    Services::AccountService.new(Repositories::AccountRepository.new(db),
                                 Repositories::TransactionRepository.new(db))
  end
end
