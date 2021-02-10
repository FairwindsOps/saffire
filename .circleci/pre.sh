#!/bin/bash

set -e

docker cp ./ e2e-command-runner:/kuiper
