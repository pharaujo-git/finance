# frozen_string_literal: true

# The two unauthenticated probes. The service document names this backend so an
# operator can tell which of the five answered a request.
class ProbesController < ApplicationController
  def health = render(plain: "ok")

  def index
    render_api({ "service" => "FinanceTracker API (Rails)", "status" => "ok",
                 "docs" => "/swagger" })
  end
end
