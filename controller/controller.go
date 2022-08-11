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
	isSynced  cache.InformerSynced
	queue     workqueue.RateLimitingInterface
}

func NewController(clientset kubernetes.Interface, depInformer v1.DeploymentInformer) *controller {
	c := &controller{
		clientset: clientset,
		depLister: depInformer.Lister(),
		isSynced:  depInformer.Informer().HasSynced,
		queue:     workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	depInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handleAdd,
		DeleteFunc: c.handleDelete,
	})
	return c
}
func (c *controller) Run(stopCh <-chan struct{}) {
	klog.Infoln("Starting controller: Dexpose")
	if !cache.WaitForCacheSync(stopCh, c.isSynced) {
		klog.Error("failed to sync cache")
	}

	go wait.Until(c.worker, 1*time.Second, stopCh)
	<-stopCh
}

func (c *controller) worker() {
	for c.processItem() {

	}
}

func (c *controller) processItem() bool {
	item, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		klog.Error(err)
	}

	name, ns, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Error(err)
	}

	klog.Infoln("Processing", name, ns)
	return true
}

func (c *controller) handleAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error(err)
	}
	klog.Infof("%s created.", key)
	c.queue.Add(obj)

}

func (c *controller) handleDelete(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error(err)
	}
	klog.Infof("%s deleted.", key)
	c.queue.Add(obj)
}
