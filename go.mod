module github.com/ogre0403/iscsi-provisioner

go 1.15

require (
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/kubernetes-sigs/sig-storage-lib-external-provisioner v4.0.0+incompatible
	github.com/ogre0403/iscsi-target-api v0.0.0-20210112015357-9489d62f8b6f
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	k8s.io/klog v1.0.0 // indirect
	k8s.io/api v0.0.0-20190814101207-0772a1bdf941
    k8s.io/apimachinery v0.0.0-20190814100815-533d101be9a6
    k8s.io/client-go v0.0.0-20190816061517-44c2c549a534
    k8s.io/utils v0.0.0-20190809000727-6c36bc71fc4a // indirect
    sigs.k8s.io/sig-storage-lib-external-provisioner v4.0.0+incompatible // indirect
)
