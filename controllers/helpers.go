// Copyright 2020 FairwindsOps Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllers

import (
	"fmt"
	"strings"

	"github.com/thoas/go-funk"
)

// parseImageString returns the repository and version of the image string
func parseImageString(image string) (string, string, error) {
	parsed := strings.Split(image, ":")
	if len(parsed) != 2 {
		return "", "", fmt.Errorf("could not parse image string: %s", image)
	}
	return parsed[0], parsed[1], nil
}

// getNewImage returns a new image string by finding a new repository from the list
// of equivalentRepositories and appending the version to it.
func getNewImage(image string, equivalentRepositories []string) (string, error) {
	oldRepository, version, err := parseImageString(image)
	if err != nil {
		return "", err
	}

	if !funk.ContainsString(equivalentRepositories, oldRepository) {
		return "", fmt.Errorf("image repository was not found in equivalentRepositories")
	}

	for _, newRepository := range equivalentRepositories {
		if newRepository != oldRepository {
			return fmt.Sprintf("%s:%s", newRepository, version), nil
		}
	}
	return "", fmt.Errorf("unable to find next repository to use")
}
