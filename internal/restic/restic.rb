#!/usr/bin/env ruby

require "json"
require "open3"

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

    def init! =  run!("init", "--insecure-no-password")

    private

    def run!(*args, env: @env)
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

begin
    case ENV["COMMAND"]
    when "init"
        puts Restic.new(ENV).init!
    else
        raise "Unknown command"
    end
rescue StandardError => e
    puts({error: e.message}.to_json)
    exit 1
end
