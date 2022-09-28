package main

import (
	"flag"
	"fmt"
	"github.com/hmsayem/dexpose/controller"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"time"
)

func getClientset() (*kubernetes.Clientset, error) {
	kubeconfig := flag.String("kubeconfig", os.Getenv("KUBE_CONFIG"), "location of kubeconfig file")
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Println(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func main() {
	clientset, err := getClientset()
	if err != nil {
		fmt.Println(err)
	}
	informerFactory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)
	depInformer := informerFactory.Apps().V1().Deployments()

	ch := make(chan struct{})
	c := controller.NewController(clientset, depInformer)
	informerFactory.Start(ch)
	c.Run(ch)
}
