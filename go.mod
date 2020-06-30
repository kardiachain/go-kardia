module github.com/kardiachain/go-kardiamain

go 1.13

require (
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/allegro/bigcache v1.2.1 // indirect
	github.com/aristanetworks/goarista v0.0.0-20190712234253-ed1100a1c015
	github.com/btcsuite/btcd v0.0.0-20190807005414-4063feeff79a
	github.com/cespare/cp v1.1.1 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/deckarep/golang-set v1.7.1
	github.com/docker/docker v17.12.0-ce-rc1.0.20200531234253-77e06fda0c94+incompatible // indirect
	github.com/ebuchman/fail-test v0.0.0-20170303061230-95f809107225
	github.com/ethereum/go-ethereum v1.9.15
	github.com/fjl/memsize v0.0.0-20190710130421-bcb5799ab5e5 // indirect
	github.com/go-kit/kit v0.10.0 // indirect
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/go-stack/stack v1.8.0
	github.com/golang/protobuf v1.3.2
	github.com/golang/snappy v0.0.1
	github.com/google/cel-go v0.3.2
	github.com/gorilla/mux v1.7.3
	github.com/hashicorp/golang-lru v0.5.4
	github.com/huin/goupnp v1.0.0
	github.com/influxdata/influxdb1-client v0.0.0-20191209144304-8bf82d3c094d
	github.com/jackpal/go-nat-pmp v1.0.2-0.20160603034137-1fa385a6f458
	github.com/mattn/go-colorable v0.1.2 // indirect
	github.com/pebbe/zmq4 v1.0.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/tsdb v0.10.0 // indirect
	github.com/rjeczalik/notify v0.9.2 // indirect
	github.com/rs/cors v1.7.0
	github.com/shirou/gopsutil v2.20.5+incompatible
	github.com/status-im/keycard-go v0.0.0-20190424133014-d95853db0f48 // indirect
	github.com/stretchr/testify v1.4.0
	github.com/syndtr/goleveldb v1.0.1-0.20190923125748-758128399b1d
	github.com/tidwall/pretty v1.0.1 // indirect
	github.com/tyler-smith/go-bip39 v1.0.2 // indirect
	github.com/xdg/scram v0.0.0-20180814205039-7eeb5667e42c // indirect
	github.com/xdg/stringprep v1.0.1-0.20180714160509-73f8eece6fdc // indirect
	go.mongodb.org/mongo-driver v1.1.0
	golang.org/x/crypto v0.0.0-20200311171314-f7b00557c8c4
	google.golang.org/genproto v0.0.0-20191115221424-83cc0476cb11
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127
	gopkg.in/yaml.v2 v2.2.8
	gotest.tools/v3 v3.0.2 // indirect
	//github.com/libopenstorage/stork v2.4.2+incompatible
	k8s.io/api v0.17.0
	k8s.io/client-go v0.17.0
	k8s.io/kubectl v0.0.0
	k8s.io/kubernetes v1.18.5
)

replace (
	github.com/Sirupsen/logrus => github.com/sirupsen/logrus v1.6.0
	github.com/codegangsta/cli => github.com/urfave/cli v1.22.4
	github.com/containerd/containerd => github.com/containerd/containerd v1.3.1-0.20200512144102-f13ba8f2f2fd
	github.com/docker/docker => github.com/docker/docker v17.12.0-ce-rc1.0.20200310163718-4634ce647cf2+incompatible
	github.com/h2non/gock => gopkg.in/h2non/gock.v1 v1.0.15
	github.com/hashicorp/go-immutable-radix => github.com/tonistiigi/go-immutable-radix v0.0.0-20170803185627-826af9ccf0fe
	github.com/heptio/ark => github.com/vmware-tanzu/velero v1.4.0
	github.com/influxdb/influxdb => github.com/influxdata/influxdb v1.8.0
	github.com/jaguilar/vt100 => github.com/tonistiigi/vt100 v0.0.0-20190402012908-ad4c4a574305
	github.com/kubernetes-incubator/external-storage => github.com/kubernetes-incubator/external-storage v5.5.0+incompatible
	github.com/mholt/caddy => github.com/caddyserver/caddy v1.0.5
	github.com/renstrom/dedent => github.com/lithammer/dedent v1.1.0
	github.com/uber-go/atomic => go.uber.org/atomic v1.6.0

)

// Dependencies for k8s.io/kubernetes
replace (
	k8s.io/api => k8s.io/kubernetes/staging/src/k8s.io/api v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/apiextensions-apiserver => k8s.io/kubernetes/staging/src/k8s.io/apiextensions-apiserver v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/apimachinery => k8s.io/kubernetes/staging/src/k8s.io/apimachinery v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/apiserver => k8s.io/kubernetes/staging/src/k8s.io/apiserver v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/cli-runtime => k8s.io/kubernetes/staging/src/k8s.io/cli-runtime v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/client-go => k8s.io/kubernetes/staging/src/k8s.io/client-go v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/cloud-provider => k8s.io/kubernetes/staging/src/k8s.io/cloud-provider v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/cluster-bootstrap => k8s.io/kubernetes/staging/src/k8s.io/cluster-bootstrap v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/code-generator => k8s.io/kubernetes/staging/src/k8s.io/code-generator v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/component-base => k8s.io/kubernetes/staging/src/k8s.io/component-base v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/cri-api => k8s.io/kubernetes/staging/src/k8s.io/cri-api v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/csi-translation-lib => k8s.io/kubernetes/staging/src/k8s.io/csi-translation-lib v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/kube-aggregator => k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/kube-controller-manager => k8s.io/kubernetes/staging/src/k8s.io/kube-controller-manager v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/kube-proxy => k8s.io/kubernetes/staging/src/k8s.io/kube-proxy v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/kube-scheduler => k8s.io/kubernetes/staging/src/k8s.io/kube-scheduler v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/kubectl => k8s.io/kubernetes/staging/src/k8s.io/kubectl v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/kubelet => k8s.io/kubernetes/staging/src/k8s.io/kubelet v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/legacy-cloud-providers => k8s.io/kubernetes/staging/src/k8s.io/legacy-cloud-providers v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/metrics => k8s.io/kubernetes/staging/src/k8s.io/metrics v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/node-api => k8s.io/kubernetes/staging/src/k8s.io/node-api v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/sample-apiserver => k8s.io/kubernetes/staging/src/k8s.io/sample-apiserver v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/sample-cli-plugin => k8s.io/kubernetes/staging/src/k8s.io/sample-cli-plugin v0.0.0-20190623232353-8c3b7d7679cc
	k8s.io/sample-controller => k8s.io/kubernetes/staging/src/k8s.io/sample-controller v0.0.0-20190623232353-8c3b7d7679cc
)
