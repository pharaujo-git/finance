# frozen_string_literal: true

# Base controller: authentication, the shared render helper, and the error
# mapping. Handlers below it do HTTP only -- every rule lives in a service.
class ApplicationController < ActionController::API
  MISSING_TOKEN_DETAIL = "Authentication is required."
  BAD_TOKEN_DETAIL = "The access token is invalid or has expired."
  MISSING_CALLER_DETAIL = "The access token does not identify a user."

  UUID_PATTERN = /\A[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\z/i

  # Declared broadest first: Rails searches rescue_from handlers in reverse
  # registration order, so the last matching one registered wins. With
  # StandardError last it would swallow every domain error as a 500.
  rescue_from StandardError, with: :render_unexpected
  rescue_from Domain::Errors::AppError, with: :render_app_error
  rescue_from Domain::Errors::FieldErrors, with: :render_field_errors

  private

  def container = Rails.configuration.x.container

  def db = container.db

  # Validates the bearer token and returns the caller's id.
  def caller_id
    @caller_id ||= begin
      header = request.headers["Authorization"].to_s
      raise Domain::Errors.unauthorized(MISSING_TOKEN_DETAIL) unless header.start_with?("Bearer ")

      raw = header.delete_prefix("Bearer ").strip
      raise Domain::Errors.unauthorized(MISSING_TOKEN_DETAIL) if raw.empty?

      begin
        container.tokens.validate(raw).user_id
      rescue Core::Security::InvalidToken
        raise Domain::Errors.unauthorized(BAD_TOKEN_DETAIL)
      end
    end
  end

  # The equivalent of the .NET routes' {id:guid} constraint: a segment that is
  # not a uuid matches no route at all, so it answers a bare 404 with no body
  # rather than reaching the database and failing there.
  def path_id
    value = params[:id].to_s
    raise ActionController::RoutingError, "not a uuid" unless UUID_PATTERN.match?(value)

    value
  end

  def body_params = request.request_parameters

  # Writes a payload with the renderer that keeps money and enums exact.
  def render_api(payload, status: :ok)
    render plain: Api::Rendering.dump(payload), status: status,
           content_type: Domain::Errors::VALIDATION_CONTENT_TYPE
  end

  def render_no_content = head(:no_content)

  def render_field_errors(error)
    render plain: Api::Rendering.dump(Domain::Errors.validation_body(error.errors)),
           status: :bad_request, content_type: Domain::Errors::VALIDATION_CONTENT_TYPE
  end

  def render_app_error(error)
    response.set_header("WWW-Authenticate", "Bearer") if error.kind == :unauthorized
    body = Domain::Errors.problem_body(error.status, error.title, error.message, request.path)
    render plain: Api::Rendering.dump(body), status: error.status,
           content_type: Domain::Errors::PROBLEM_CONTENT_TYPE
  end

  def render_unexpected(error)
    # A bad {id} is raised as a routing error so it lands here as a bare 404,
    # with no body, exactly as an unmatched route would.
    return head(:not_found) if error.is_a?(ActionController::RoutingError)

    Rails.logger.error("unhandled error: #{error.class}: #{error.message}")
    body = Domain::Errors.problem_body(500, "Internal Server Error",
                                       "An unexpected error occurred.", request.path)
    render plain: Api::Rendering.dump(body), status: :internal_server_error,
           content_type: Domain::Errors::PROBLEM_CONTENT_TYPE
  end
end
