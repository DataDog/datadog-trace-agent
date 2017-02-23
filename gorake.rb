
def go_build(program, opts={})
  default_cmd = "go build -a"
  if ENV["INCREMENTAL_BUILD"] then
    default_cmd = "go build -i"
  end
  opts = {
    :cmd => default_cmd,
    :race => false
  }.merge(opts)

  dd = 'main'
  commit = `git rev-parse --short HEAD`.strip
  branch = `git rev-parse --abbrev-ref HEAD`.strip
  date = `date +%FT%T%z`.strip
  goversion = `go version`.strip
  agentversion = ENV["TRACE_AGENT_VERSION"] || "0.99.0"

  ldflags = {
    "#{dd}.BuildDate" => "#{date}",
    "#{dd}.GitCommit" => "#{commit}",
    "#{dd}.GitBranch" => "#{branch}",
    "#{dd}.GoVersion" => "#{goversion}",
    "#{dd}.Version" => "#{agentversion}",
  }.map do |k,v|
    if goversion.include?("1.4")
      "-X #{k} '#{v}'"
    else
      "-X '#{k}=#{v}'"
    end
  end.join ' '

  cmd = opts[:cmd]
  cmd += ' -race' if opts[:race]

  sh "#{cmd} -ldflags \"#{ldflags}\" #{program}"
end


def go_lint(path)
  out = `golint #{path}/*.go`
  errors = out.split("\n")
  puts "#{errors.length} linting issues found"
  if errors.length > 0
    puts out
    fail
  end
end

def go_vet(path)
  sh "go vet #{path}"
end

def go_test(path, opts = {})
  cmd = 'go test'
  filter = ''
  if opts[:coverage_file]
    cmd += " -coverprofile=#{opts[:coverage_file]} -coverpkg=./..."
    filter = "2>&1 | grep -v 'warning: no packages being tested depend on'" # ugly hack
  end
  sh "#{cmd} #{path} #{filter}"
end

# return the dependencies of all the packages who start with the root path
def go_pkg_deps(pkgs, root_path)
  deps = []
  pkgs.each do |pkg|
    deps << pkg
    `go list -f '{{ join .Deps "\\n"}}' #{pkg}`.split("\n").select do |path|
      if path.start_with? root_path
        deps << path
      end
    end
  end
  return deps.sort.uniq
end

def go_fmt(path)
  out = `go fmt ./...`
  errors = out.split("\n")
  if errors.length > 0
    puts out
    fail
  end
end

