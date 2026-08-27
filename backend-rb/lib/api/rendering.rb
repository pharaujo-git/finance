# frozen_string_literal: true

module Api
  # JSON rendering with exact control over how each scalar reaches the wire.
  #
  # The stock generator will not do. Money must arrive as a *bare* JSON number
  # that keeps the scale it carries (1250.00, not 1250.0 and not "1250.00"),
  # which no Ruby Float can represent. So the document is written directly, and
  # a Money renders from its own digits.
  module Rendering
    module_function

    def dump(payload)
      out = +""
      write(payload, out)
      out
    end

    def write(value, out)
      case value
      when nil then out << "null"
      when true then out << "true"
      when false then out << "false"
      when Domain::Money then out << value.to_s
      when Domain::Enums::Value
        # An ordinal naming no member is written back as a bare number, exactly
        # as JsonStringEnumConverter does.
        out << (value.defined? ? value.wire_name.to_json : value.ordinal.to_s)
      when Domain::Instant then out << value.to_s.to_json
      when ::Integer then out << value.to_s
      when ::String then out << value.to_json
      when ::Time then out << Domain::Instant.from_time(value).to_s.to_json
      when ::Array then write_array(value, out)
      when ::Hash then write_hash(value, out)
      else out << value.to_json
      end
    end

    def write_array(value, out)
      out << "["
      value.each_with_index do |item, index|
        out << "," if index.positive?
        write(item, out)
      end
      out << "]"
    end

    def write_hash(value, out)
      out << "{"
      first = true
      value.each do |key, item|
        out << "," unless first
        first = false
        out << key.to_s.to_json << ":"
        write(item, out)
      end
      out << "}"
    end
  end
end
