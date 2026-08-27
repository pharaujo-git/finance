# frozen_string_literal: true

class AuthController < ApplicationController
  # Register and login are the only anonymous routes in the API.
  def register
    body = Api::Schemas.register(body_params)
    render_api(service.register(body[:email], body[:password], body[:name]))
  end

  def login
    body = Api::Schemas.login(body_params)
    render_api(service.login(body[:email], body[:password]))
  end

  def profile = render_api(service.profile(caller_id))

  def update_profile
    body = Api::Schemas.update_profile(body_params)
    render_api(service.update_profile(caller_id, body[:name], body[:currency]))
  end

  private

  def service = Services::AuthService.new(Repositories::UserRepository.new(db), container.tokens)
end
