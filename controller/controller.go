package controller

import (
	"context"
	"fmt"
	coreapi "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netapi "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (c *controller) handleAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error(err)
	}
	klog.Infof("%v created.", key)
	c.queue.Add(obj)
}

func (c *controller) handleDelete(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Error(err)
	}
	klog.Infof("%v deleted.", key)
	c.queue.Add(obj)
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
	defer c.queue.Forget(item)
	if shutdown {
		return false
	}

	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		klog.Error(err)
	}

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Error(err)
	}

	dep, err := c.clientset.AppsV1().Deployments(ns).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("Deployment deleted: %v/%v", name, ns)
			return true
		}
		klog.Errorf("Failed to get Deployment: %v/%v. Reason: %v", name, ns, err)
		return false
	}

	if err := c.syncDeployment(dep); err != nil {
		klog.Errorf("Failed to sync Deployment: %v/%v. Reason: %v", name, ns, err)
		return false
	}
	return true
}

func (c *controller) syncDeployment(dep *coreapi.Deployment) error {
	klog.Infof("Syncing Deployment: %v/%v", dep.Name, dep.Namespace)
	if err := c.expose(dep); err != nil {
		return err
	}
	return nil
}

func (c *controller) expose(dep *coreapi.Deployment) error {
	svc, err := c.createService(dep.Name, dep.Namespace, dep.Labels)
	if err != nil {
		return err
	}
	if err = c.createIngress(svc); err != nil {
		return err
	}
	return nil
}

func (c *controller) createService(name, ns string, labels map[string]string) (*corev1.Service, error) {
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
			},
		},
	}
	s, err := c.clientset.CoreV1().Services(ns).Create(context.Background(), &svc, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	klog.Infof("Service created: %v/%v", name, ns)
	return s, nil
}

func (c *controller) createIngress(svc *corev1.Service) error {
	pathType := netapi.PathTypePrefix
	ingress := netapi.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/rewrite-target": "/",
			},
		},
		Spec: netapi.IngressSpec{
			Rules: []netapi.IngressRule{
				{
					IngressRuleValue: netapi.IngressRuleValue{
						HTTP: &netapi.HTTPIngressRuleValue{
							Paths: []netapi.HTTPIngressPath{
								{
									Backend: netapi.IngressBackend{
										Service: &netapi.IngressServiceBackend{
											Name: svc.Name,
											Port: netapi.ServiceBackendPort{
												Number: 80,
											},
										},
									},
									Path:     fmt.Sprintf("/%s", svc.Name),
									PathType: &pathType,
								},
							},
						},
					},
				},
			},
		},
	}
	_, err := c.clientset.NetworkingV1().Ingresses(svc.Namespace).Create(context.Background(), &ingress, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	klog.Infof("Ingress created: %v/%v", ingress.Name, ingress.Name)
	return nil
}
