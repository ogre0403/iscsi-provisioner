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
	"github.com/ogre0403/iscsi-target-api/pkg/cfg"
	"github.com/ogre0403/iscsi-target-api/pkg/client"
	"strconv"
	"strings"
	"time"

	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/util"
	"github.com/powerman/rpc-codec/jsonrpc2"
	"github.com/spf13/viper"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//var log = logrus.New()

type chapSessionCredentials struct {
	InUser      string `properties:"node.session.auth.username"`
	InPassword  string `properties:"node.session.auth.password"`
	OutUser     string `properties:"node.session.auth.username_in"`
	OutPassword string `properties:"node.session.auth.password_in"`
}

//initiator_set_auth(initiator_wwn, in_user, in_pass, out_user, out_pass)
type initiatorSetAuthArgs struct {
	InitiatorWwn string `json:"initiator_wwn"`
	InUser       string `json:"in_user"`
	InPassword   string `json:"in_pass"`
	OutUser      string `json:"out_user"`
	OutPassword  string `json:"out_pass"`
}

type volDestroyArgs struct {
	Pool string `json:"pool"`
	Name string `json:"name"`
}

type exportDestroyArgs struct {
	Pool         string `json:"pool"`
	Vol          string `json:"vol"`
	InitiatorWwn string `json:"initiator_wwn"`
}

type iscsiProvisioner struct {
	targetdURL  string
	iscsiClient *client.Client
}

type export struct {
	InitiatorWwn string `json:"initiator_wwn"`
	Lun          int32  `json:"lun"`
	VolName      string `json:"vol_name"`
	VolSize      int    `json:"vol_size"`
	VolUUID      string `json:"vol_uuid"`
	Pool         string `json:"pool"`
}

// NewiscsiProvisioner creates new iscsi provisioner
func NewiscsiProvisioner(addr string, port int) controller.Provisioner {

	return &iscsiProvisioner{
		iscsiClient: client.NewClient(addr, port),
	}
}

// getAccessModes returns access modes iscsi volume supported.
func (p *iscsiProvisioner) getAccessModes() []v1.PersistentVolumeAccessMode {
	return []v1.PersistentVolumeAccessMode{
		v1.ReadWriteOnce,
		v1.ReadOnlyMany,
	}
}

// Provision creates a storage asset and returns a PV object representing it.
func (p *iscsiProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	if !util.AccessModesContainedInAll(p.getAccessModes(), options.PVC.Spec.AccessModes) {
		return nil, fmt.Errorf("invalid AccessModes %v: only AccessModes %v are supported", options.PVC.Spec.AccessModes, p.getAccessModes())
	}
	log.V(2).Infof("new provision request received for pvc: ", options.PVName)
	iqn, err := p.createVolume(options)
	if err != nil {
		log.Errorf("Create volume fail: %s", err.Error())
		return nil, err
	}

	pv := &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:   options.PVName,
			Labels: map[string]string{},
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			// set volumeMode from PVC Spec
			VolumeMode: options.PVC.Spec.VolumeMode,
			PersistentVolumeSource: v1.PersistentVolumeSource{
				ISCSI: &v1.ISCSIPersistentVolumeSource{
					TargetPortal: options.Parameters["targetPortal"],
					IQN:          iqn,
					//ISCSIInterface: options.Parameters["iscsiInterface"],
					Lun:      1, // todo: support multiple LUNs
					ReadOnly: getReadOnly(options.Parameters["readonly"]),
					FSType:   getFsType(options.Parameters["fsType"]),
					//Portals:           portals, todo multipath
					//DiscoveryCHAPAuth: getBool(options.Parameters["chapAuthDiscovery"]), // todo: support CHAP
					//SessionCHAPAuth:   getBool(options.Parameters["chapAuthSession"]), // todo: support CHAP
					//SecretRef:         getSecretRef(getBool(options.Parameters["chapAuthDiscovery"]), getBool(options.Parameters["chapAuthSession"]), &v1.SecretReference{Name: viper.GetString("provisioner-name") + "-chap-secret"}), // todo: support CHAP
				},
			},
		},
	}
	return pv, nil
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

func getSecretRef(discovery bool, session bool, ref *v1.SecretReference) *v1.SecretReference {
	if discovery || session {
		return ref
	}
	return nil
}

func getBool(value string) bool {
	res, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return res

}

func (p *iscsiProvisioner) Delete(volume *v1.PersistentVolume) error {
	//volume.Spec.PersistentVolumeSource.ISCSI.
	//volume.
	//target := &cfg.TargetCfg{
	//	TargetIQN: fmt.Sprintf("iqn.%s-%s.k8s.%s:%s", "2021", "01", options.PVC.Namespace, options.PVC.Name),
	//	TargetIQN: fmt.Sprintf("iqn."),
	//}
	//
	//if err := p.iscsiClient.DeleteTarget(target); err != nil {
	//	return err
	//}

	return nil
}

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *iscsiProvisioner) _Delete(volume *v1.PersistentVolume) error {
	//vol from the annotation
	log.V(2).Infof("volume deletion request received: ", volume.GetName())
	for _, initiator := range strings.Split(volume.Annotations["initiators"], ",") {
		log.V(2).Infof("removing iscsi export: ", volume.Annotations["volume_name"], volume.Annotations["pool"], initiator)
		err := p.exportDestroy(volume.Annotations["volume_name"], volume.Annotations["pool"], initiator)
		if err != nil {
			log.Error(err)
			return err
		}
		log.V(2).Infof("iscsi export removed: ", volume.Annotations["volume_name"], volume.Annotations["pool"], initiator)
	}
	log.V(2).Infof("removing logical volume : ", volume.Annotations["volume_name"], volume.Annotations["pool"])
	err := p.volDestroy(volume.Annotations["volume_name"], volume.Annotations["pool"])
	if err != nil {
		log.Error(err)
		return err
	}
	log.V(2).Infof("logical volume removed: ", volume.Annotations["volume_name"], volume.Annotations["pool"])
	log.V(2).Infof("volume deletion request completed: ", volume.GetName())
	return nil
}

//func initLog() {
//	var err error
//	log.Level, err = logrus.ParseLevel(viper.GetString("log-level"))
//	if err != nil {
//		log.Fatalln(err)
//	}
//}

func (p *iscsiProvisioner) createVolume(options controller.VolumeOptions) (string, error) {

	target := fmt.Sprintf("iqn.%d-%02d.k8s.%s:%s",
		time.Now().Year(), time.Now().Month(),
		options.PVC.Namespace, options.PVC.Name)

	v := &cfg.VolumeCfg{
		Size: fmt.Sprintf("%dm", getSize(options)/1024/1024),
		Path: options.PVC.Namespace,
		Name: options.PVName,
	}
	if err := p.iscsiClient.CreateVolume(v); err != nil {
		return "", err
	}

	lun := &cfg.LunCfg{
		TargetIQN: target,
		Volume:    v,
	}

	if err := p.iscsiClient.AttachLun(lun); err != nil {
		return "", err
	}
	log.V(2).Infof("volume created with target and size: ", lun.TargetIQN, v.Size)

	return target, nil
}

func getSize(options controller.VolumeOptions) int64 {
	q := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	return q.Value()
}

func (p *iscsiProvisioner) getVolumeName(options controller.VolumeOptions) string {
	return options.PVName
}

func (p *iscsiProvisioner) getVolumeGroup(options controller.VolumeOptions) string {
	if options.Parameters["volumeGroup"] == "" {
		return "vg-targetd"
	}
	return options.Parameters["volumeGroup"]
}

func (p *iscsiProvisioner) getInitiators(options controller.VolumeOptions) []string {
	return strings.Split(options.Parameters["initiators"], ",")
}

// volDestroy removes calls vol_destroy targetd API to remove volume.
func (p *iscsiProvisioner) volDestroy(vol string, pool string) error {
	client, err := p.getConnection()
	defer client.Close()
	if err != nil {
		log.Error(err)
		return err
	}
	args := volDestroyArgs{
		Pool: pool,
		Name: vol,
	}
	err = client.Call("vol_destroy", args, nil)
	return err
}

// exportDestroy calls export_destroy targetd API to remove export of volume.
func (p *iscsiProvisioner) exportDestroy(vol string, pool string, initiator string) error {
	client, err := p.getConnection()
	defer client.Close()
	if err != nil {
		log.Error(err)
		return err
	}
	args := exportDestroyArgs{
		Pool:         pool,
		Vol:          vol,
		InitiatorWwn: initiator,
	}
	err = client.Call("export_destroy", args, nil)
	return err
}

//initiator_set_auth(initiator_wwn, in_user, in_pass, out_user, out_pass)

func (p *iscsiProvisioner) setInitiatorAuth(initiator string, inUser string, inPassword string, outUser string, outPassword string) error {

	client, err := p.getConnection()
	defer client.Close()
	if err != nil {
		log.Error(err)
		return err
	}

	//make arguments object
	args := initiatorSetAuthArgs{
		InitiatorWwn: initiator,
		InUser:       inUser,
		InPassword:   inPassword,
		OutUser:      outUser,
		OutPassword:  outPassword,
	}
	//call remote procedure with args
	err = client.Call("initiator_set_auth", args, nil)
	return err
}

func (p *iscsiProvisioner) getConnection() (*jsonrpc2.Client, error) {
	log.V(2).Infof("opening connection to targetd: ", p.targetdURL)

	client := jsonrpc2.NewHTTPClient(p.targetdURL)
	if client == nil {
		log.Error("error creating the connection to targetd", p.targetdURL)
		return nil, errors.New("error creating the connection to targetd")
	}
	log.V(2).Infof("targetd client created")
	return client, nil
}

func (p *iscsiProvisioner) SupportsBlock() bool {
	return true
}
