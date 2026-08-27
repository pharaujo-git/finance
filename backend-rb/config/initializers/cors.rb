# frozen_string_literal: true

# Exact-match origin echo, mirroring the .NET policy: WithOrigins(list) with any
# header and any method, and no credentials support. Every OPTIONS answers 204,
# matched origin or not.
class CorsMiddleware
  def initialize(app)
    @app = app
  end

  def call(env)
    origin = env["HTTP_ORIGIN"]
    allowed = Rails.configuration.x.container&.settings&.allowed_origins || []
    headers = {}

    if origin && allowed.include?(origin)
      headers["Access-Control-Allow-Origin"] = origin
      headers["Vary"] = "Origin"

      if env["REQUEST_METHOD"] == "OPTIONS"
        headers["Access-Control-Allow-Methods"] = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
        requested = env["HTTP_ACCESS_CONTROL_REQUEST_HEADERS"]
        headers["Access-Control-Allow-Headers"] = requested.to_s.empty? ? "*" : requested
        headers["Access-Control-Max-Age"] = "86400"
      end
    end

    return [ 204, headers, [] ] if env["REQUEST_METHOD"] == "OPTIONS"

    status, response_headers, body = @app.call(env)
    [ status, response_headers.merge(headers), body ]
  end
end

Rails.application.config.middleware.insert_before 0, CorsMiddleware
