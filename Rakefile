require "./gorake.rb"

desc 'Bootstrap CI environment'
task :bootstrap do
  tools = {
    'github.com/golang/lint' => {
      version: 'b8599f7d71e7fead76b25aeb919c0e2558672f4a',
      main_pkg: './golint',
      check_cmd: 'golint',
      clean_cmd: "rm -rf #{ENV['GOPATH']}/src/golang.org/x"
    }
  }

  tools.each do |pkg, info|
    if info[:check_cmd]
      has_cmd = false
      begin
        sh "which #{info[:check_cmd]}"
        has_cmd = true
      rescue
        sh info[:clean_cmd] if info[:clean_cmd]
      end
      next if has_cmd
    end
    path = "#{ENV['GOPATH']}/src/#{pkg}"
    FileUtils.rm_rf(path)

    sh "go get #{pkg}"
    sh "cd #{path} && git reset --hard #{info[:version]} && go install #{info[:main_pkg]}"
  end

  sh 'go get github.com/Masterminds/glide'
end

desc 'Restore code workspace to known state from locked versions'
task :restore => [:bootstrap] do
  sh 'glide install'
end

PACKAGES = %w(
  ./agent
  ./config
  ./fixtures
  ./model
  ./quantile
  ./quantizer
  ./sampler
  ./statsd
  ./watchdog
)

EXCLUDE_LINT = [
    'model/services_gen.go',
    'model/trace_gen.go',
    'model/span_gen.go',
]

task :default => [:ci]

desc "Build Datadog Trace agent"
task :build do
  go_build("github.com/DataDog/datadog-trace-agent/agent", {
    :cmd => "go build -a -o trace-agent",
    :race => ENV['GO_RACE'] == 'true',
    :add_build_vars => ENV['TRACE_AGENT_ADD_BUILD_VARS'] != 'false'
  })
end

desc "Build Datadog Trace agent for windows"
task :windows do
  ["386", "amd64"].each do |arch|
    go_build("github.com/DataDog/datadog-trace-agent/agent", {
               :cmd => "GOOS=windows GOARCH=#{arch} go build -a -o trace-agent-windows-#{arch}.exe",
               :race => ENV['GO_RACE'] == 'true'
             })
  end
end

desc "Install Datadog Trace agent"
task :install do
  go_build("github.com/DataDog/datadog-trace-agent/agent", :cmd=>"go build -i -o $GOPATH/bin/trace-agent")
end

desc "Test Datadog Trace agent"
task :test do
  PACKAGES.each { |pkg| go_test(pkg) }
end

desc "Test Datadog Trace agent"
task :coverage do
  files = []
  i = 1
  PACKAGES.each do |pkg|
    file = "#{i}.coverage"
    files << file
    go_test(pkg, {:coverage_file => file})
    i += 1
  end
  files.select! {|f| File.file? f}

  sh "gocovmerge #{files.join(' ')} >|tests.coverage"
  sh "rm #{files.join(' ')}"

  sh 'go tool cover -html=tests.coverage'
end

desc "Run Datadog Trace agent"
task :run do
  ENV['DD_APM_ENABLED'] = 'true'
  sh "./trace-agent -config ./agent/trace-agent.ini"
end

task :lint do
  error = false
  PACKAGES.each do |pkg|
    puts "golint #{pkg}"
    output = `golint #{pkg}`.split("\n")
    output = output.reject do |line|
      filename = line.split(':')[0]
      EXCLUDE_LINT.include?(filename)
    end
    if !output.empty?
      puts output
      error = true
    end
  end
  fail "We have some linting errors" if error
end

task :vet do
  PACKAGES.each { |pkg| go_vet(pkg) }
end

task :fmt do
  PACKAGES.each { |pkg| go_fmt(pkg) }
end

# FIXME: add :test in the list
desc "Datadog Trace agent CI script (fmt, vet, etc)"
task :ci => [:fmt, :vet, :lint, :test, :build]

task :err do
  sh "errcheck github.com/DataDog/datadog-trace-agent"
end
