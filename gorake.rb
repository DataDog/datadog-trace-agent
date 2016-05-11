
def go_build(program, opts={})
  opts = {
    :cmd => "go build -a"
  }.merge(opts)

  dd = 'main'
  commit = `git rev-parse --short HEAD`.strip
  branch = `git rev-parse --abbrev-ref HEAD`.strip
  date = `date`.strip
  goversion = `go version`.strip
  agentversion = "1.0.0-unreleased"

  ldflags = "-X #{dd}.BuildDate '#{date}' -X #{dd}.GitCommit '#{commit}' -X #{dd}.GitBranch '#{branch}' -X #{dd}.GoVersion '#{goversion}' -X #{dd}.Version '#{agentversion}'"

  cmd = opts[:cmd]
  sh "#{cmd} -ldflags \"#{ldflags}\" #{program}"
end

def go_errcheck(path_or_paths)
  paths = path_or_paths.is_a?(String) ? [path_or_paths] : path_or_paths
  # errcheck with some sane ignores
  # don't bother checking these
  ignores = [
    "github.com/cihub/seelog:.*",
    "github.com/DataDog/datadog-go/statsd:Count|Gauge|Histogram"
  ]

  sh "errcheck -ignore \"#{ignores.join(',')}\" #{paths.join(" ")}"
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

def go_test(path, opts={})
  opts = {
    :v => false,
    :include => "raclette"
  }.merge(opts)

  paths = [path]
  if opts[:include]
    deps = `go list -f '{{ join .Deps "\\n"}}' #{path} | sort | uniq`.split("\n").select do |p|
      p.include? opts[:include]
    end
    paths = paths.concat(deps)
  end

  v = opts[:v] ? "-v" : ""
  sh "go test #{v} #{paths.join(' ')}"
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

