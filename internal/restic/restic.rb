require "json"
require "open3"

RESTIC_KEY_PREFIX = "RESTIC_KEY_"
RESTIC_KEY_MAIN = "RESTIC_KEY_MAIN"

class ResticError < StandardError
    attr_reader :stdout, :stderr, :status

    def initialize(stderr, status)
        super("Code: #{status.exitstatus}. #{stderr[:message]}")
        @stderr = stderr
        @status = status
    end
end

class Restic
    def initialize(env)
        @env = env
    end

    def init!
        begin
            run!("init")
        rescue ResticError => e
            raise if e.stderr[:code] != 1
            raise if !e.message.include?("config file already exists")
            raise if !import?
            import!
        end

        add_keys!
    end

    private

    def import!
        puts("Importing repository")
        raise "Not implemented"
    end

    def add_keys!
        puts("Adding keys")
        raise "Not implemented"
    end

    def keys = @env.
                    select { _1.start_with?(RESTIC_KEY_PREFIX) && _1 != "RESTIC_KEY_MAIN" }.
                    transform_keys { _1.delete_prefix(RESTIC_KEY_PREFIX).downcase }

    def import? = @env["RESTIC_IMPORT"] || false

    def run!(args)
        stdout, stderr, status = Open3.capture3("restic", *args, "--json")
        raise ResticError.new(JSON.parse(stderr, symbolize_names: true), status) if !status.success?

        JSON.parse(stdout, symbolize_names: true)
    end
end



puts Restic.new(ENV).init!
