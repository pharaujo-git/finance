# frozen_string_literal: true

# Numbered so it runs before the container initializer, which needs these
# constants at boot.
require Rails.root.join("lib/finance")
