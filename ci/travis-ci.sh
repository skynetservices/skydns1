#!/bin/bash

COMMAND=$1
EXIT_VALUE=0

##
# SCRIPT COMMANDS
##

# before-install
#
# Do some stuff before npm install
#
before-install() {
  echo
}

# install
#
# Override install
#
install() {
  go get github.com/goraft/raft
  go get github.com/gorilla/mux
  go get github.com/miekg/dns
  go get github.com/skynetservices/skydns1/msg
  go get github.com/rcrowley/go-metrics
  go get github.com/stathat/go
  go get github.com/codegangsta/cli
  go get github.com/rcrowley/go-metrics/influxdb
  go get -a -d -v -ldflags '-s' ./...
  go build -v ./...
}

#$ node -pe 'JSON.parse(process.argv[1]).foo' "$(cat foobar.json)"

# before-script
#
# Setup Drupal to run the tests.
#
before-script() {
  echo
}

# script
#
# Run the tests.
#
script() {
  cd $TRAVIS_BUILD_DIR
  go test -v ./...
}

# after-script
#
# Clean up after the tests.
#
after-script() {
  echo
}

# after-success
#
# Clean up after the tests.
#
after-success() {
  go install . ./...
  $HOME/gopath/bin/skydns1 --help
}

# before-deploy
#
# Clean up after the tests.
#
before-deploy() {
  echo
}

# after-deploy
#
# Clean up after the tests.
#
after-deploy() {
  echo
}

##
# UTILITY FUNCTIONS:
##

# Sets the exit level to error.
set_error() {
  EXIT_VALUE=1
}

# Runs a command and sets an error if it fails.
run_command() {
  set -xv
  if ! $@; then
    set_error
  fi
  set +xv
}

##
# SCRIPT MAIN:
##

# Capture all errors and set our overall exit value.
trap 'set_error' ERR

# We want to always start from the same directory:
cd $TRAVIS_BUILD_DIR

case $COMMAND in
  before-install)
    run_command before-install
    ;;

  install)
    run_command install
    ;;

  before-script)
    run_command before-script
    ;;

  script)
    run_command script
    ;;

  after-script)
    run_command after_script
    ;;

  after-success)
    run_command after-success
    ;;

  before-deploy)
    run_command before-deploy
    ;;

  after-deploy)
    run_command after-deploy
    ;;
esac

exit $EXIT_VALUE
