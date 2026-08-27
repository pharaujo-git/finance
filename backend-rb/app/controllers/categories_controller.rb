# frozen_string_literal: true

# There is deliberately no GET by id here.
class CategoriesController < ApplicationController
  def index = render_api(service.list_all(caller_id))
  def create = render_api(service.create(caller_id, Api::Schemas.category(body_params)))

  def update
    render_api(service.update(caller_id, path_id, Api::Schemas.category(body_params)))
  end

  def destroy
    service.remove(caller_id, path_id)
    render_no_content
  end

  private

  def service = Services::CategoryService.new(Repositories::CategoryRepository.new(db))
end
