# frozen_string_literal: true

Rails.application.routes.draw do
  get "/health", to: "probes#health"
  root to: "probes#index"

  scope :api do
    scope :auth do
      post "register", to: "auth#register"
      post "login", to: "auth#login"
      get "me", to: "auth#profile"
      put "me", to: "auth#update_profile"
    end

    resources :accounts, only: %i[index show create update destroy]
    resources :categories, only: %i[index create update destroy]

    # /export and /import are declared before the :id member routes so the
    # literal segments are not swallowed by the parameter.
    scope :transactions do
      get "export", to: "transactions#export"
      post "import", to: "transactions#import"
    end
    resources :transactions, only: %i[index show create update destroy]

    resources :recurring, only: %i[index create update destroy], controller: "recurring"
    resources :budgets, only: %i[index create update destroy]

    resources :goals, only: %i[index create update destroy] do
      member { post "contribute" }
    end

    scope :dashboard do
      get "summary", to: "analytics#summary"
      get "networth", to: "analytics#networth"
      get "cashflow", to: "analytics#cashflow"
      get "spending", to: "analytics#spending"
    end
    scope :reports do
      get "monthly", to: "analytics#monthly_report"
      get "categories", to: "analytics#category_report"
    end
  end
end
