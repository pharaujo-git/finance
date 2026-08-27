# Vercel entry point.
#
# Vercel's Ruby runtime is WEBrick-based: it loads this file and calls `Handler`
# with a WEBrick request and response, not with a Rack env. Rails is a Rack
# application, so the two are bridged here by hand rather than through
# Rackup::Handler::WEBrick, which wants a live WEBrick server object that this
# runtime never constructs.
#
# The app itself is untouched: `config/environment.rb` boots exactly what puma
# boots in the container, and `Rails.application` is the same Rack app.
#
# Unlike the Python runtime, this one preserves the real request path, so no
# rewrite trickery is needed -- `req.path` really is `/api/auth/login`.

require 'stringio'
require_relative '../config/environment'

# Header names WEBrick reports that map to bare CGI keys rather than HTTP_ ones.
UNPREFIXED_HEADERS = { 'content-type' => 'CONTENT_TYPE', 'content-length' => 'CONTENT_LENGTH' }.freeze

def rack_env_for(req)
  body = req.body.to_s

  env = {
    'REQUEST_METHOD' => req.request_method,
    'SCRIPT_NAME' => '',
    'PATH_INFO' => req.path,
    'QUERY_STRING' => req.query_string.to_s,
    'SERVER_NAME' => (req.host || 'localhost'),
    'SERVER_PORT' => (req.port || 443).to_s,
    'SERVER_PROTOCOL' => 'HTTP/1.1',
    'rack.input' => StringIO.new(body),
    'rack.errors' => $stderr,
    # Vercel terminates TLS in front of this process, and production.rb sets
    # assume_ssl, so the app must be told the original scheme was https or
    # force_ssl would bounce every request.
    'rack.url_scheme' => 'https'
  }

  req.each do |name, value|
    key = name.to_s.downcase
    env[UNPREFIXED_HEADERS[key] || "HTTP_#{key.tr('-', '_').upcase}"] = value
  end
  env['CONTENT_LENGTH'] ||= body.bytesize.to_s unless body.empty?

  env
end

Handler = lambda do |req, res|
  status, headers, body = Rails.application.call(rack_env_for(req))

  res.status = status
  headers.each do |name, value|
    # Rack 3 allows an array of values for one header (Set-Cookie, mainly).
    res[name] = value.is_a?(Array) ? value.join("\n") : value
  end

  chunks = +''
  body.each { |chunk| chunks << chunk }
  body.close if body.respond_to?(:close)
  res.body = chunks
end
