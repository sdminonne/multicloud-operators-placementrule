// Copyright 2019 The Kubernetes Authors.
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

package utils

import (
	"encoding/base64"
	"strings"

	appv1 "github.com/open-cluster-management/multicloud-operators-placementrule/pkg/apis/apps/v1"
	rbacv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clusterv1alpha1 "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
)

func FilteClustersByIdentity(authClient kubernetes.Interface, object runtime.Object, clmap map[string]*clusterv1alpha1.Cluster) error {
	objmeta, err := meta.Accessor(object)
	if err != nil {
		return nil
	}

	objanno := objmeta.GetAnnotations()
	if objanno == nil {
		return nil
	}

	if _, ok := objanno[appv1.UserIdentityAnnotation]; !ok {
		return nil
	}

	var clusters []*clusterv1alpha1.Cluster

	for _, cl := range clmap {
		clusters = append(clusters, cl.DeepCopy())
	}

	clusters = filterClusterByUserIdentity(object, clusters, authClient, "deployables", "create")
	validclMap := make(map[string]bool)

	for _, cl := range clusters {
		validclMap[cl.GetName()] = true
	}

	for k := range clmap {
		if valid, ok := validclMap[k]; !ok || !valid {
			delete(clmap, k)
		}
	}

	return nil
}

// filterClusterByUserIdentity filters cluster by checking if user can act on on resources
func filterClusterByUserIdentity(
	obj runtime.Object,
	clusters []*clusterv1alpha1.Cluster,
	kubeclient kubernetes.Interface,
	resource, verb string,
) []*clusterv1alpha1.Cluster {
	if kubeclient == nil {
		return clusters
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return clusters
	}

	annotations := accessor.GetAnnotations()
	if annotations == nil {
		return clusters
	}

	filteredClusters := []*clusterv1alpha1.Cluster{}

	for _, cluster := range clusters {
		user, groups := extractUserAndGroup(annotations)
		sar := &rbacv1.SubjectAccessReview{
			Spec: rbacv1.SubjectAccessReviewSpec{
				ResourceAttributes: &rbacv1.ResourceAttributes{
					Namespace: cluster.Namespace,
					Group:     "apps.open-cluster-management.io",
					Verb:      verb,
					Resource:  resource,
				},
				User:   user,
				Groups: groups,
			},
		}
		result, err := kubeclient.AuthorizationV1().SubjectAccessReviews().Create(sar)

		if err != nil {
			continue
		}

		if !result.Status.Allowed {
			continue
		}

		filteredClusters = append(filteredClusters, cluster)
	}

	return filteredClusters
}
func extractUserAndGroup(annotations map[string]string) (string, []string) {
	var user string

	var groups []string

	encodedUser, ok := annotations[appv1.UserIdentityAnnotation]
	if ok {
		decodedUser, err := base64.StdEncoding.DecodeString(encodedUser)
		if err == nil {
			user = string(decodedUser)
		}
	}

	encodedGroups, ok := annotations[appv1.UserGroupAnnotation]
	if ok {
		decodedGroup, err := base64.StdEncoding.DecodeString(encodedGroups)
		if err == nil {
			groups = strings.Split(string(decodedGroup), ",")
		}
	}

	return user, groups
}
