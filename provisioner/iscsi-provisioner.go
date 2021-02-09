/*
Copyright 2016 The Kubernetes Authors.

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

package provisioner

import (
	"errors"
	"fmt"
	log "github.com/golang/glog"
	"github.com/ogre0403/iscsi-target-client/pkg/client"
	"github.com/ogre0403/iscsi-target-client/pkg/model"
	"strconv"
	"strings"
	"time"

	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/util"
	"github.com/spf13/viper"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	VolumeGroup    = "volumeGroup"
	VolumeType     = "volumeType"
	ACL            = "acl"
	TargetPortal   = "targetPortal"
	ThinPool       = "thinPool"
	AnnotationThin = "iscsi-provisioner/thin"
	AnnotationChap = "iscsi-provisioner/chap"
	VolumeTypeLVM  = "lvm"
)

type iscsiProvisioner struct {
	iscsiClient *client.Client
}

// NewiscsiProvisioner creates new iscsi provisioner
func NewiscsiProvisioner(addr string, config *model.ServerCfg) controller.Provisioner {

	return &iscsiProvisioner{
		iscsiClient: client.NewClient(addr, config),
	}
}

// Provision creates a storage asset and returns a PV object representing it.
func (p *iscsiProvisioner) Provision(options controller.ProvisionOptions) (*v1.PersistentVolume, error) {
	if !util.AccessModesContainedInAll(p.getAccessModes(), options.PVC.Spec.AccessModes) {
		return nil, fmt.Errorf("invalid AccessModes %v: only AccessModes %v are supported", options.PVC.Spec.AccessModes, p.getAccessModes())
	}
	log.V(2).Infof("new provision request received for pvc: %s/%s", options.PVC.Namespace, options.PVC.Name)
	iqn, err := p.createVolume(options)
	if err != nil {
		log.Errorf("Create volume fail: %s", err.Error())
		return nil, err
	}

	var portals []string
	if len(options.StorageClass.Parameters["portals"]) > 0 {
		portals = strings.Split(options.StorageClass.Parameters["portals"], ",")
	}

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:   options.PVName,
			Labels: map[string]string{},
			Annotations: map[string]string{
				VolumeType:  options.StorageClass.Parameters[VolumeType],
				VolumeGroup: options.StorageClass.Parameters[VolumeGroup],
			},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: *options.StorageClass.ReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			// set volumeMode from PVC Spec
			VolumeMode: options.PVC.Spec.VolumeMode,
			PersistentVolumeSource: v1.PersistentVolumeSource{
				ISCSI: &v1.ISCSIPersistentVolumeSource{
					TargetPortal:    options.StorageClass.Parameters[TargetPortal],
					IQN:             iqn,
					Lun:             1, // todo: support multiple LUNs
					ReadOnly:        getReadOnly(options.StorageClass.Parameters["readonly"]),
					FSType:          getFsType(options.StorageClass.Parameters["fsType"]),
					Portals:         portals,
					SessionCHAPAuth: getBool(options.PVC.Annotations[AnnotationChap]),
					SecretRef:       getSecretRef(getBool(options.PVC.Annotations[AnnotationChap])),
				},
			},
		},
	}
	return pv, nil
}

// Delete removes the storage asset that was created by Provision represented by the given PV.
func (p *iscsiProvisioner) Delete(volume *v1.PersistentVolume) error {
	log.V(2).Infof("new volume deletion request received for pv: %s", volume.GetName())

	target := &model.Target{
		TargetIQN: volume.Spec.ISCSI.IQN,
	}
	if err := p.iscsiClient.DeleteTarget(target); err != nil {
		log.Errorf("Delete target %s fail: %s", target.TargetIQN, err.Error())
		return err
	}
	log.V(2).Infof("target %s removed ", target.TargetIQN)

	vol := &model.Volume{
		Type:  volume.Annotations[VolumeType],
		Group: volume.Annotations[VolumeGroup],
		Name:  volume.Name,
	}

	if err := p.iscsiClient.DeleteVolume(vol); err != nil {
		log.Errorf("Remove volume %s/%s fail: %s", vol.Group, vol.Name, err.Error())
		return err
	}
	log.V(2).Infof("volume file <VOLUME_IMAGE_ROOT>/%s/%s is removed ", vol.Group, vol.Name)

	log.V(2).Infof("volume %s deletion request completed. ", volume.GetName())
	return nil
}

func (p *iscsiProvisioner) SupportsBlock() bool {
	return true
}

func (p *iscsiProvisioner) createVolume(options controller.ProvisionOptions) (string, error) {

	target := fmt.Sprintf("iqn.%d-%02d.k8s.%s:%s",
		time.Now().Year(), time.Now().Month(),
		options.PVC.Namespace, options.PVC.Name)

	vType := options.StorageClass.Parameters[VolumeType]
	v := &model.Volume{
		Type:  vType,
		Group: options.StorageClass.Parameters[VolumeGroup],
		Name:  options.PVName,
		Size:  uint64(getSize(options)),
		Unit:  "B",
	}

	thinPool, isPoolFound := options.StorageClass.Parameters[ThinPool]
	isThinValue, isThinFound := options.PVC.Annotations[AnnotationThin]
	isthinvalue, _ := strconv.ParseBool(isThinValue)

	if isThinFound && isthinvalue {

		if vType == VolumeTypeLVM && (!isPoolFound || thinPool == "") {
			return "", errors.New(fmt.Sprintf(
				"LVM volume %s desire thin provision, but thin pool name is not defined in parameters of storage class %s",
				options.PVName, options.StorageClass.Name))
		}

		v.ThinProvision = true
		v.ThinPool = thinPool
	}

	if err := p.iscsiClient.CreateVolume(v); err != nil {
		return "", err
	}

	lun := &model.Lun{
		TargetIQN:  target,
		Volume:     v,
		EnableChap: getBool(options.PVC.Annotations[AnnotationChap]),
	}
	if len(options.StorageClass.Parameters[ACL]) > 0 {
		lun.AclIpList = strings.Split(options.StorageClass.Parameters[ACL], ",")
	} else {
		lun.AclIpList = []string{}
	}

	if err := p.iscsiClient.AttachLun(lun); err != nil {
		return "", err
	}
	log.V(2).Infof("volume created with target %s and size %s: ", lun.TargetIQN, v.Size)

	return target, nil
}

// getAccessModes returns access modes iscsi volume supported.
func (p *iscsiProvisioner) getAccessModes() []v1.PersistentVolumeAccessMode {
	return []v1.PersistentVolumeAccessMode{
		v1.ReadWriteOnce,
		v1.ReadOnlyMany,
	}
}

func getReadOnly(readonly string) bool {
	isReadOnly, err := strconv.ParseBool(readonly)
	if err != nil {
		return false
	}
	return isReadOnly
}

func getFsType(fsType string) string {
	if fsType == "" {
		return viper.GetString("default-fs")
	}
	return fsType
}

func getSize(options controller.ProvisionOptions) int64 {
	q := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	return q.Value()
}

func getBool(value string) bool {
	res, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return res
}

func getSecretRef(session bool) *v1.SecretReference {
	if session {
		return &v1.SecretReference{Name: viper.GetString("provisioner-name") + "-chap-secret"}
	}
	return nil
}
