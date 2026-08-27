# frozen_string_literal: true

module Domain
  # The DataAnnotations rules the .NET API enforces, reproduced message for
  # message. The frontend renders these strings verbatim.
  #
  # Note the asymmetry copied from .NET's resources: `required` and
  # `email_address` put the field name first ("The Email field ..."), while the
  # length and range rules put "The field" first ("The field Email ...").
  module Validation
    # ModelState keys are the PascalCase property names of the .NET DTOs.
    FIELD = {
      email: "Email", password: "Password", name: "Name", currency: "Currency",
      icon: "Icon", color: "Color", description: "Description", notes: "Notes",
      amount: "Amount", month: "Month", limit: "Limit",
      target_amount: "TargetAmount", current_amount: "CurrentAmount",
      page: "Page", page_size: "PageSize", search: "Search"
    }.freeze

    # The key MVC's JSON reader uses for a body-level failure.
    JSON_BODY_FIELD = "$"

    LIMITS = {
      email: 256, password_min: 8, password_max: 128, name: 200, currency: 8,
      icon: 64, color: 32, description: 500, notes: 2000, search: 200
    }.freeze

    # Looser than a real month parse on purpose: "2026-13" passes here, and
    # fails in the service.
    MONTH_PATTERN = /\A\d{4}-\d{2}\z/

    module_function

    def required_message(field) = "The #{field} field is required."
    def email_address_message(field) = "The #{field} field is not a valid e-mail address."

    def min_length_message(field, limit)
      "The field #{field} must be a string or array type with a minimum length of '#{limit}'."
    end

    def max_length_message(field, limit)
      "The field #{field} must be a string or array type with a maximum length of '#{limit}'."
    end

    def range_message(field, minimum, maximum)
      "The field #{field} must be between #{minimum} and #{maximum}."
    end

    def missing_members_message(members)
      "The JSON payload was missing required properties, including the following: #{members.join(', ')}"
    end

    def invalid_value_message(value, field) = "The value '#{value}' is not valid for #{field}."

    # Whitespace-only counts as missing, as [Required] does.
    def required(errs, field, value)
      errs.add(field, required_message(field)) if value.to_s.strip.empty?
    end

    # Exactly one '@', neither first nor last character, on the raw value.
    def email_address(errs, field, value)
      text = value.to_s
      at = text.index("@")
      valid = !at.nil? && at.positive? && at == text.rindex("@") && at < text.length - 1
      errs.add(field, email_address_message(field)) unless valid
    end

    def min_length(errs, field, value, limit)
      errs.add(field, min_length_message(field, limit)) if value.to_s.length < limit
    end

    def max_length(errs, field, value, limit)
      # Counted in characters, matching the rune count the Go backend uses.
      errs.add(field, max_length_message(field, limit)) if value.to_s.length > limit
    end

    # A missing value is not this rule's business; the required-member check
    # owns it.
    def money_range(errs, field, value, minimum)
      return if value.nil?

      low = Money.bound(minimum)
      high = Money.bound(Money::MAX)
      errs.add(field, range_message(field, minimum, Money::MAX)) if value < low || value > high
    end

    def int_range(errs, field, value, minimum, maximum)
      return if value.nil?

      errs.add(field, range_message(field, minimum, maximum)) if value < minimum || value > maximum
    end

    # Reports absent `required` members under "$", returning true when
    # something was missing. Every caller returns immediately on true, so no
    # other rule runs -- the .NET pipeline fails deserialisation before model
    # validation starts, and that short-circuit is the single most important
    # ordering behaviour here.
    def required_members(errs, missing)
      return false if missing.empty?

      errs.add(JSON_BODY_FIELD, missing_members_message(missing))
      true
    end
  end
end
