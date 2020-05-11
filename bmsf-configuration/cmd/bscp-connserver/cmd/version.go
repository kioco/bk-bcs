/*
Tencent is pleased to support the open source community by making Blueking Container Service available.
Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
Licensed under the MIT License (the "License"); you may not use this file except
in compliance with the License. You may obtain a copy of the License at
http://opensource.org/licenses/MIT
Unless required by applicable law or agreed to in writing, software distributed under
the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
either express or implied. See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"github.com/spf13/cobra"

	"bk-bscp/internal/version"
)

func genVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show bk-bscp-connserver version",
		Run: func(cmd *cobra.Command, args []string) {
			version.ShowVersion()
		},
	}
}