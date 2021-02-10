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
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getNewImage(t *testing.T) {
	type args struct {
		image                  string
		equivalentRepositories []string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "basic",
			args: args{
				image: "quay.io/Company/OldRepository:v3.0.0",
				equivalentRepositories: []string{
					"quay.io/Company/OldRepository",
					"Company/NewRepository",
				},
			},
			want:    "Company/NewRepository:v3.0.0",
			wantErr: false,
		},
		{
			name: "error repo not in list",
			args: args{
				image: "quay.io/Company/OldRepository:v3.0.0",
				equivalentRepositories: []string{
					"Company/NewRepository",
				},
			},
			wantErr: true,
		},
		{
			name: "bad image passed",
			args: args{
				image: "notanimagestring",
				equivalentRepositories: []string{
					"Company/NewRepository",
				},
			},
			wantErr: true,
		},
		{
			name: "only one image",
			args: args{
				image: "Company/NewRepository:v3.0.0",
				equivalentRepositories: []string{
					"Company/NewRepository",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getNewImage(tt.args.image, tt.args.equivalentRepositories)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, tt.want, got)
			}
		})
	}
}

func Test_parseImageString(t *testing.T) {

	tests := []struct {
		name    string
		image   string
		want    string
		want1   string
		wantErr bool
	}{
		{
			name:    "basic",
			image:   "quay.io/fairwinds/test:v1.0.0",
			want:    "quay.io/fairwinds/test",
			want1:   "v1.0.0",
			wantErr: false,
		},
		{
			name:    "error",
			image:   "notanimagestring",
			wantErr: true,
		},
		{
			name:    "basic2",
			image:   "fairwinds/test:1.0.0",
			want:    "fairwinds/test",
			want1:   "1.0.0",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := parseImageString(tt.image)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
				assert.Equal(t, tt.want1, got1)
			}
		})
	}
}
