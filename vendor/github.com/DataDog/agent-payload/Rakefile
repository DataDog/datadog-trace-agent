#
# Rakefile for agent-payload
#

protoc_binary="protoc"
protoc_version="3.5.1"

namespace :codegen do

  task :install_protoc do
    if `bash -c "protoc --version"` != "libprotoc ${protoc_version}"
      protoc_binary="/tmp/protoc#{protoc_version}"
      sh <<-EOF
        /bin/bash <<BASH
        if [ ! -f #{protoc_binary} ] ; then
          echo "Downloading protoc #{protoc_version}"
          cd /tmp
          if [ "$(uname -s)" = "Darwin" ] ; then
            curl -OL https://github.com/google/protobuf/releases/download/v#{protoc_version}/protoc-#{protoc_version}-osx-x86_64.zip
          else
            curl -OL https://github.com/google/protobuf/releases/download/v#{protoc_version}/protoc-#{protoc_version}-linux-x86_64.zip
          fi
          unzip protoc-#{protoc_version}*.zip
          mv bin/protoc #{protoc_binary}
        fi
BASH
      EOF
    end
  end

  task :protoc => ['install_protoc'] do
    sh <<-EOF
      /bin/bash <<BASH
      set -euo pipefail
      #{protoc_binary} --proto_path=$GOPATH/src:. --gogofast_out=. --java_out=java     agent_logs_payload.proto
      #{protoc_binary} --proto_path=$GOPATH/src:. --gogofast_out=. --python_out=python agent_payload.proto
      # Go files will be written to the folder defined in the 'go_package' option.
      # So we need to move them into the correct folder.
      cp -r ./github.com/DataDog/agent-payload/* .
      rm -rf ./github.com/
BASH
    EOF
  end

  desc 'Run all code generators.'
  multitask :all => [:protoc]

end

desc "Run all code generation."
task :codegen => ['codegen:all']

desc "Run all protobuf code generation."
task :protobuf => ['codegen:protoc']
