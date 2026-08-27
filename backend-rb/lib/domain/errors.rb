# frozen_string_literal: true

module Domain
  # Domain errors and the two problem shapes the API renders.
  #
  # Both were captured from the .NET API and reproduced by the Go, Python and
  # Node ones; the frontend's readError() parses them, so the field names, the
  # status titles and even the content types are fixed.
  module Errors
    PROBLEM_CONTENT_TYPE = "application/problem+json"
    # MVC content-negotiates plain JSON, not problem+json, for validation 400s.
    VALIDATION_CONTENT_TYPE = "application/json; charset=utf-8"

    VALIDATION_PROBLEM_TYPE = "https://tools.ietf.org/html/rfc9110#section-15.5.1"
    VALIDATION_PROBLEM_TITLE = "One or more validation errors occurred."

    # Mirrors ApiExceptionHandler.Responses in the .NET API.
    STATUS_TITLES = {
      validation: [ 400, "Bad Request" ],
      unauthorized: [ 401, "Unauthorized" ],
      not_found: [ 404, "Not Found" ],
      conflict: [ 409, "Conflict" ]
    }.freeze

    # A domain failure carrying the kind that decides its HTTP status.
    class AppError < StandardError
      attr_reader :kind

      def initialize(kind, message)
        @kind = kind
        super(message)
      end

      def status = STATUS_TITLES.fetch(kind).first
      def title = STATUS_TITLES.fetch(kind).last
    end

    # Field-keyed errors, rendered as MVC's validation dictionary.
    class FieldErrors < StandardError
      attr_reader :errors

      def initialize(errors = {})
        @errors = errors
        super(VALIDATION_PROBLEM_TITLE)
      end

      def add(field, message)
        (errors[field] ||= []) << message
      end

      def empty? = errors.empty?

      # Raises itself when anything was collected.
      def raise_if_any = empty? ? nil : raise(self)
    end

    module_function

    def validation(message) = AppError.new(:validation, message)
    def unauthorized(message) = AppError.new(:unauthorized, message)
    def conflict(message) = AppError.new(:conflict, message)

    # Takes the entity name, not the sentence: "Account" -> "Account was not found.".
    def not_found(entity) = AppError.new(:not_found, "#{entity} was not found.")

    # An RFC 9457 problem document. `type` is absent because the .NET handler
    # never sets it; the readable text lands in `detail`.
    def problem_body(status, title, detail, instance = nil)
      body = { "title" => title, "status" => status, "detail" => detail }
      body["instance"] = instance if instance && !instance.empty?
      body
    end

    # The field-error dictionary MVC writes for a 400. Keys are sorted because
    # Go marshals a map with sorted keys, and the bodies are compared byte for
    # byte -- so they sort by codepoint, not by locale.
    def validation_body(errors)
      {
        "type" => VALIDATION_PROBLEM_TYPE,
        "title" => VALIDATION_PROBLEM_TITLE,
        "status" => 400,
        "errors" => errors.keys.sort.to_h { |key| [ key, errors[key] ] }
      }
    end
  end
end
