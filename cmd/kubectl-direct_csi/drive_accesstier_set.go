/*
 * This file is part of MinIO Direct CSI
 * Copyright (C) 2021, MinIO, Inc.
 *
 * This code is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, version 3,
 * as published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License, version 3,
 * along with this program.  If not, see <http://www.gnu.org/licenses/>
 *
 */

package main

import (
	"context"
	"fmt"

	directcsi "github.com/minio/direct-csi/pkg/apis/direct.csi.min.io/v1beta1"
	"github.com/minio/direct-csi/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var accessTierSet = &cobra.Command{
	Use:   "set [hot|cold|warm]",
	Short: "tag DirectCSI drive(s) based on their access-tiers [hot,cold,warm]",
	Long:  "",
	Example: `
# Sets the 'access-tier:cold' tag to all the 'Available' DirectCSI drives 
$ kubectl direct-csi drives access-tier set cold --all

# Sets the 'access-tier:warm' tag to all nvme drives in all nodes 
$ kubectl direct-csi drives access-tier set warm --drives=/dev/nvme*

# Sets the 'access-tier:hot' tag to all drives from a particular node
$ kubectl direct-csi drives access-tier set hot --nodes=directcsi-1

# Combine multiple parameters using multi-arg
$ kubectl direct-csi drives access-tier set hot --nodes=directcsi-1 --nodes=othernode-2 --status=ready

# Combine multiple parameters using csv
$ kubectl direct-csi drives access-tier set hot --nodes=directcsi-1,othernode-2 --status=ready
`,
	RunE: func(c *cobra.Command, args []string) error {
		return setAccessTier(c.Context(), args)
	},
	Aliases: []string{},
}

func init() {
	accessTierSet.PersistentFlags().StringSliceVarP(&drives, "drives", "d", drives, "glob selector for drive paths")
	accessTierSet.PersistentFlags().StringSliceVarP(&nodes, "nodes", "n", nodes, "glob selector for node names")
	accessTierSet.PersistentFlags().BoolVarP(&all, "all", "a", all, "tag all available drives")
	accessTierSet.PersistentFlags().StringSliceVarP(&status, "status", "s", status, "glob prefix match for drive status")
}

func setAccessTier(ctx context.Context, args []string) error {
	dryRun := viper.GetBool(dryRunFlagName)
	if !all {
		if len(drives) == 0 && len(nodes) == 0 && len(status) == 0 {
			return fmt.Errorf("atleast one of '%s', '%s', '%s' or '%s' should be specified", utils.Bold("--all"), utils.Bold("--drives"), utils.Bold("--nodes"), utils.Bold("--status"))
		}
	}

	if len(args) != 1 {
		return fmt.Errorf("Invalid input arguments. Please use '%s' for examples to set access-tiers", utils.Bold("--help"))
	}

	accessT, err := utils.ValidateAccessTier(args[0])
	if err != nil {
		return err
	}

	utils.Init()

	directClient := utils.GetDirectCSIClient()
	driveList, err := directClient.DirectCSIDrives().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	if len(driveList.Items) == 0 {
		glog.Errorf("No resource of %s found\n", bold("DirectCSIDrive"))
		return fmt.Errorf("No resources found")
	}

	filterDrives := []directcsi.DirectCSIDrive{}
	for _, d := range driveList.Items {
		if d.MatchGlob(nodes, drives, status) {
			filterDrives = append(filterDrives, d)
		}
	}

	for _, d := range filterDrives {
		if d.Status.DriveStatus == directcsi.DriveStatusUnavailable {
			continue
		}
		d.Status.AccessTier = accessT
		if dryRun {
			if err := utils.LogYAML(d); err != nil {
				return err
			}
			continue
		}
		if _, err := directClient.DirectCSIDrives().Update(ctx, &d, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}
