require "json"
require "open3"

RESTIC_KEY_PREFIX = "RESTIC_KEY_"
RESTIC_KEY_MAIN = "RESTIC_KEY_MAIN"

class ResticError < StandardError
    attr_reader :stdout, :stderr, :status

    def initialize(stderr, status)
        stderr = JSON.parse(stderr, symbolize_names: true)

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
            return import! if import?
            raise
        end

        add_keys!
    end

    private

    def import!
        keys.each_with_object({}) do |(name, key), acc|
            stored_keys = run!(["key", "list"], env: { "RESTIC_PASSWORD" => key})

            if stored_keys.count != keys.count
                raise StandardError, "Stored keys count (#{stored_keys.count}) does not match provided count (#{keys.count}), cannot import"
            end


            acc[name] = stored_keys.find { _1[:current]}[:id]
        end
    end

    def add_keys!
        stored_keys = run!(["key", "list"])

        raise StandardError, "Newly created repository has #{stored_keys.count} keys, expected 1" if stored_keys.count > 1

        keys.each_with_object({}) do |(name, key), acc|
            next if name == "main"

            id = add_key!(key)
            acc[name] = id
        end.merge(main: stored_keys.first[:id])
    end

    def keys = @env.
                    select { _1.start_with?(RESTIC_KEY_PREFIX) }.
                    transform_keys { _1.delete_prefix(RESTIC_KEY_PREFIX).downcase }

    def import? = @env["RESTIC_IMPORT"] || false

    def run!(args, env: @env)
        stdout, stderr, status = Open3.capture3(env, "restic", *args, "--json")
        raise ResticError.new(stderr, status) if !status.success?

        JSON.parse(stdout, symbolize_names: true)
    end

    def add_key!(key)
        # 'key add' does not support json output
        Open3.popen3("restic", "key", "add", "--new-password-file", "/dev/stdin") do |stdin, stdout, stderr, thread|
            stdin.write(key)
            stdin.close

            # parse "saved new key with ID 50f679876d02713ccfdf9f65290372f02cede2df0e7b958d677268d9f8b1300f"
            return stdout.read.match(/saved new key with ID (.*)/)[1] if thread.value.exitstatus.zero?

            throw ResticError.new(stderr.read, thread.value.exitstatus)
        end
    end
end

case ENV["COMMAND"]
when "init"
    puts Restic.new(ENV).init!
else
    raise "Unknown command"
end
