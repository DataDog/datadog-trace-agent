require "./gorake.rb"

desc 'Bootstrap CI environment'
task :bootstrap do
  groot = `go env GOROOT`
  sh 'go get github.com/robfig/glock'
  # Don't get vet in the dev environment
  unless groot and groot.include? "/usr/"
    sh 'go get golang.org/x/tools/cmd/vet'
  end
end

desc 'Restore from glockfile'
task :restore => [:bootstrap] do
  sh 'glock sync github.com/DataDog/raclette'
end

PACKAGES = %w(
  ./agent
  ./model
  ./tracegen
)

task :default => [:ci]

desc "Build Raclette agent"
task :build do
  go_build("github.com/DataDog/raclette/agent", :cmd => "go build -a -o raclette")
  go_build("github.com/DataDog/raclette/tracegen", :cmd => "go build -a -o generator")
end

desc "Install Raclette agent"
task :install do
  go_build("github.com/DataDog/raclette/agent", :cmd=>"go install")
  go_build("github.com/DataDog/raclette/tracegen", :cmd=>"go install")
end

desc "Test Raclette agent"
task :test do go_test("./raclette") end

desc "Run Raclette agent"
task :run do
  sh "./raclette -config ./agent/trace-agent.ini -log_config ./seelog.xml"
end

task :lint do
  PACKAGES.each do |pkg|
    find_cmd = "find #{pkg} -name '*.go'"
    # for the top level dir, only lint file at this level
    find_cmd += ' -depth 1' if pkg == '.'

    sh "#{find_cmd} | xargs golint"
  end
end

task :vet do
  PACKAGES.each { |pkg| go_vet(pkg) }
end

task :fmt do
  PACKAGES.each { |pkg| go_fmt(pkg) }
end

# FIXME: add :test in the list
desc "Raclette agent CI script (fmt, vet, etc)"
task :ci => [:restore, :fmt, :vet, :lint, :build]

task :err do
  sh "errcheck github.com/DataDog/raclette"
end
