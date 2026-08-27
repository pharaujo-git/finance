# frozen_string_literal: true

# Loads the layered code in dependency order.
#
# These files are required rather than autoloaded -- see the note in
# config/application.rb. The order below is the dependency order: domain first,
# then the ports that use it, then the services, then the HTTP helpers.
require "bigdecimal"
require "date"
require "json"

require_relative "domain/money"
require_relative "domain/enums"
require_relative "domain/instant"
require_relative "domain/dates"
require_relative "domain/errors"
require_relative "domain/validation"

require_relative "core/database"
require_relative "core/security"

require_relative "repositories/rows"
require_relative "repositories/repositories"

require_relative "services/balance"
require_relative "services/services"
require_relative "services/recurring"
require_relative "services/analytics"
require_relative "services/csv_io"

require_relative "api/rendering"
require_relative "api/schemas"
