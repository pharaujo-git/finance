# frozen_string_literal: true

# Everything a handler needs, resolved once at boot.
#
# The settings are read from the environment with the same names and fallbacks
# the other four backends use, so one deployment environment drives any of them.
module Finance
  class ConfigError < StandardError; end

  # Must stay byte-identical to JwtOptions.LocalDevelopmentSecret in the .NET
  # API, or locally issued tokens stop crossing backends.
  LOCAL_DEVELOPMENT_SECRET = "finance-tracker-local-development-signing-key-please-override"
  DEFAULT_ALLOWED_ORIGINS = "http://localhost:5173"
  # Keeps this API off the .NET (5000), Go (8081), Python (8082) and Node (8083) ports.
  DEFAULT_PORT = 8084

  Settings = Struct.new(:database_url, :jwt_secret, :port, :allowed_origins, keyword_init: true)

  class Container
    attr_reader :settings, :db, :tokens

    def initialize(settings, db: nil)
      @settings = settings
      @db = db || ::Core::Database.new(settings.database_url)
      @tokens = ::Core::Security::TokenService.new(settings.jwt_secret)
    end
  end

  module_function

  # Splits the comma-separated origin list, falling back to the default.
  def parse_origins(raw)
    value = raw.to_s.strip.empty? ? DEFAULT_ALLOWED_ORIGINS : raw
    origins = value.split(",").map(&:strip).reject(&:empty?)
    origins.empty? ? [ DEFAULT_ALLOWED_ORIGINS ] : origins
  end

  # Reads the environment. DATABASE_URL is required; the rest have defaults.
  def load_settings(env = ENV)
    database_url = env["DATABASE_URL"].to_s.strip
    if database_url.empty?
      raise ConfigError, "config: DATABASE_URL is required (postgres:// connection string)"
    end

    secret = env["JWT_SECRET"].to_s.strip
    secret = LOCAL_DEVELOPMENT_SECRET if secret.empty?

    port = DEFAULT_PORT
    raw_port = env["PORT"].to_s.strip
    unless raw_port.empty?
      port = Integer(raw_port, exception: false)
      unless port&.positive? && port <= 65_535
        raise ConfigError, "config: PORT must be a TCP port number, got #{raw_port}"
      end
    end

    Settings.new(database_url: database_url, jwt_secret: secret, port: port,
                 allowed_origins: parse_origins(env["ALLOWED_ORIGINS"]))
  end
end

# The test harness builds its own container against a throwaway schema, so the
# boot-time one is skipped when a container is already installed.
Rails.application.config.to_prepare do
  next if Rails.configuration.x.container
  next if ENV["FINANCE_SKIP_BOOT_CONTAINER"] == "1"

  Rails.configuration.x.container = Finance::Container.new(Finance.load_settings)
end
