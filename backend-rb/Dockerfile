# Build the gems in one stage so the runtime image carries no compiler toolchain.
FROM ruby:4.0.5-slim AS build

# pg builds a native extension, so the client headers are needed here only.
RUN apt-get update -qq \
    && apt-get install --no-install-recommends -y build-essential libpq-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
ENV BUNDLE_PATH=/gems \
    BUNDLE_DEPLOYMENT=1 \
    BUNDLE_WITHOUT=development:test

COPY Gemfile Gemfile.lock ./
RUN bundle install \
    && rm -rf /gems/ruby/*/cache /gems/ruby/*/bundler/gems/*/.git


FROM ruby:4.0.5-slim

# libpq is the only runtime dependency; the headers stayed in the build stage.
RUN apt-get update -qq \
    && apt-get install --no-install-recommends -y libpq5 \
    && rm -rf /var/lib/apt/lists/*

ENV RAILS_ENV=production \
    BUNDLE_PATH=/gems \
    BUNDLE_DEPLOYMENT=1 \
    BUNDLE_WITHOUT=development:test \
    PORT=8084

WORKDIR /app
COPY --from=build /gems /gems
COPY . .

# Rails wants these writable; nothing else on disk is.
RUN groupadd --system --gid 1000 rails \
    && useradd --system --uid 1000 --gid 1000 --create-home rails \
    && mkdir -p log tmp \
    && chown -R rails:rails log tmp
USER rails

EXPOSE 8084

# The shell form is what expands $PORT, which the platform sets.
CMD ["sh", "-c", "exec bundle exec puma -C config/puma.rb"]
