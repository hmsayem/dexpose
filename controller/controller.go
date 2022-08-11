package controller

import (
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	lister "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"time"
)

type controller struct {
	clientset kubernetes.Interface
	depLister lister.DeploymentLister
	depCached cache.InformerSynced
	queue     workqueue.RateLimitingInterface
}

func NewController(clientset kubernetes.Interface, depInformer v1.DeploymentInformer) *controller {
	c := &controller{
		clientset: clientset,
		depLister: depInformer.Lister(),
		depCached: depInformer.Informer().HasSynced,
		queue:     workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	depInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    handleAdd,
		DeleteFunc: handleDelete,
	})
	return c
}
func (c *controller) Run(stopCh <-chan struct{}) {
	klog.Infoln("Starting controller: Dexpose")
	if !cache.WaitForCacheSync(stopCh, c.depCached) {
		klog.Error("failed to sync cache")
	}

	go wait.Until(worker, 1*time.Second, stopCh)
	<-stopCh
}

func worker() {

}
func handleAdd(obj interface{}) {
	klog.Info("Add event!")
}

func handleDelete(obj interface{}) {

}
