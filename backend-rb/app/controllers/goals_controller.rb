# frozen_string_literal: true

class GoalsController < ApplicationController
  def index = render_api(service.list_all(caller_id))
  def create = render_api(service.create(caller_id, Api::Schemas.goal(body_params)))

  def update
    render_api(service.update(caller_id, path_id, Api::Schemas.goal(body_params)))
  end

  def destroy
    service.remove(caller_id, path_id)
    render_no_content
  end

  def contribute
    body = Api::Schemas.contribute(body_params)
    render_api(service.contribute(caller_id, path_id, body[:amount]))
  end

  private

  def service = Services::GoalService.new(Repositories::GoalRepository.new(db))
end
