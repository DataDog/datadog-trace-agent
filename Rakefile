require "./gorake.rb"

desc 'Bootstrap CI environment'
task :bootstrap do
  sh 'go get github.com/robfig/glock'
end

desc 'Restore from glockfile'
task :restore => [:bootstrap] do
  sh 'glock sync github.com/DataDog/raclette'
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
)

task :default => [:ci]

desc "Build Raclette agent"
task :build do
  go_build("github.com/DataDog/raclette/agent", :cmd => "go build -a -o trace-agent")
end

desc "Install Raclette agent"
task :install do
  go_build("github.com/DataDog/raclette/agent", :cmd=>"go build -i -o $GOPATH/bin/trace-agent")
end

desc "Test Raclette agent"
task :test do go_test("./agent") end

desc "Run Raclette agent"
task :run do
  sh "./trace-agent -debug -config ./agent/trace-agent.ini"
end

desc "Build deb from current agent source"
task :build_deb do
    sh "cd packaging/ && dpkg-buildpackage -us -uc -b"
end

task :lint do
  error = false
  PACKAGES.each do |pkg|
    puts "golint #{pkg}"
    output = `golint #{pkg}`.strip
    if output.length > 0
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
desc "Raclette agent CI script (fmt, vet, etc)"
task :ci => [:restore, :fmt, :vet, :lint, :test, :build]

task :err do
  sh "errcheck github.com/DataDog/raclette"
end
