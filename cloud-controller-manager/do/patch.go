/*
Copyright 2017 DigitalOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package do

import (
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	clientset "k8s.io/client-go/kubernetes"
)

func patchService(client clientset.Interface, cur, mod *v1.Service) error {
	curJSON, err := json.Marshal(cur)
	if err != nil {
		return fmt.Errorf("failed to serialize current service object: %s", err)
	}

	modJSON, err := json.Marshal(mod)
	if err != nil {
		return fmt.Errorf("failed to serialize modified service object: %s", err)
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJSON, modJSON, v1.Service{})
	if err != nil {
		return fmt.Errorf("failed to create 2-way merge patch: %s", err)
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return nil
	}
	_, err = client.CoreV1().Services(cur.Namespace).Patch(cur.Name, types.StrategicMergePatchType, patch, "")
	if err != nil {
		return fmt.Errorf("failed to patch service object %s/%s: %s", cur.Namespace, cur.Name, err)
	}

	return nil
}
