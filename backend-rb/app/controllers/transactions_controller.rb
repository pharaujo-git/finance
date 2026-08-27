# frozen_string_literal: true

class TransactionsController < ApplicationController
  DEFAULT_PAGE = 1
  DEFAULT_PAGE_SIZE = 20

  def index
    reader = Api::QueryReader.new(request.query_parameters)
    query = {
      page: reader.number_or("page", "Page", DEFAULT_PAGE),
      page_size: reader.number_or("pageSize", "PageSize", DEFAULT_PAGE_SIZE),
      account_id: reader.identifier("accountId", "AccountId"),
      category_id: reader.identifier("categoryId", "CategoryId"),
      type: reader.enum("type", "Type", "TransactionType"),
      date_from: reader.moment("from", "From"),
      date_to: reader.moment("to", "To"),
      search: reader.text("search").to_s
    }
    reader.done

    render_api(service.search(caller_id, query))
  end

  def show = render_api(service.get(caller_id, path_id))
  def create = render_api(service.create(caller_id, Api::Schemas.transaction(body_params)))

  def update
    render_api(service.update(caller_id, path_id, Api::Schemas.transaction(body_params)))
  end

  def destroy
    service.remove(caller_id, path_id)
    render_no_content
  end

  def export
    reader = Api::QueryReader.new(request.query_parameters)
    # Lowercase field keys here, unlike the search action above -- that
    # asymmetry comes from the .NET action signatures.
    date_from = reader.moment("from", "from")
    date_to = reader.moment("to", "to")
    reader.done

    body = csv_service.export(caller_id, date_from, date_to)
    name = Services::CsvService.export_file_name(Time.now)
    response.set_header("Content-Disposition",
                        "attachment; filename=#{name}; filename*=UTF-8''#{name}")
    render plain: body, content_type: "text/csv"
  end

  def import
    file = params[:file]
    raise Domain::Errors.validation(Services::CsvService::MISSING_FILE_MESSAGE) if file.nil?

    content = file.respond_to?(:read) ? file.read : file.to_s
    if content.nil? || content.empty?
      raise Domain::Errors.validation(Services::CsvService::MISSING_FILE_MESSAGE)
    end
    if content.bytesize > Services::CsvService::MAX_UPLOAD_BYTES
      raise Domain::Errors.validation(Services::CsvService::OVERSIZE_MESSAGE)
    end

    render_api(csv_service.import(caller_id, content))
  end

  private

  def service
    Services::TransactionService.new(
      Repositories::TransactionRepository.new(db),
      Repositories::AccountRepository.new(db),
      Services::CategoryService.new(Repositories::CategoryRepository.new(db))
    )
  end

  def csv_service
    Services::CsvService.new(Repositories::TransactionRepository.new(db),
                             Repositories::AccountRepository.new(db),
                             Repositories::CategoryRepository.new(db))
  end
end
