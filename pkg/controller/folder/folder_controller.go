package folder

import (
	"context"
	"time"

	"fmt"
	"io/ioutil"
	"strings"

	csye7374v1alpha1 "github.com/advancecloud7374/s3-operator/pkg/apis/csye7374/v1alpha1"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_folder")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Folder Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileFolder{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("folder-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Folder
	err = c.Watch(&source.Kind{Type: &csye7374v1alpha1.Folder{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Folder
	// err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
	// 	IsController: true,
	// 	OwnerType:    &csye7374v1alpha1.Folder{},
	// })
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &csye7374v1alpha1.Folder{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileFolder implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileFolder{}

// ReconcileFolder reconciles a Folder object
type ReconcileFolder struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

const FolderFinalizerName = "finalizer.csye7374.com"

// Reconcile reads that state of the cluster for a Folder object and makes changes based on the state read
// and what is in the Folder.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileFolder) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Folder")

	// Fetch the Folder instance
	instance := &csye7374v1alpha1.Folder{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	//My changes

	secretCheck := &corev1.Secret{}

	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.UserSecret.Name, Namespace: instance.Namespace}, secretCheck)
	if secretCheck.Name != "" && secretCheck.Name != instance.Spec.UserSecret.Name {
		instance.Status.SetupComplete = false
		err = r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	//changes by ravi
	awsAccessKeyIDbyte, err := ioutil.ReadFile("/usr/local/etc/operator/AWS_ACCESS_KEY_ID")
	if err != nil {
		return reconcile.Result{}, err
	}
	awsAccessKeyID := strings.TrimRight(string(awsAccessKeyIDbyte), "\r\n")
	awsSecretAccessKeybyte, err := ioutil.ReadFile("/usr/local/etc/operator/AWS_SECRET_ACCESS_KEY")
	if err != nil {
		return reconcile.Result{}, err
	}
	awsSecretAccessKey := strings.TrimRight(string(awsSecretAccessKeybyte), "\r\n")
	bucketbyte, err := ioutil.ReadFile("/usr/local/etc/operator/bucketname")
	if err != nil {
		return reconcile.Result{}, err
	}
	bucket := strings.TrimRight(string(bucketbyte), "\r\n")
	regionbyte, err := ioutil.ReadFile("/usr/local/etc/operator/aws_region")
	if err != nil {
		return reconcile.Result{}, err
	}
	region := strings.TrimRight(string(regionbyte), "\r\n")
	token := ""
	creds := credentials.NewStaticCredentials(awsAccessKeyID, awsSecretAccessKey, token)

	_, err = creds.Get()
	if err != nil {
		return reconcile.Result{}, err
	}
	cfg := aws.NewConfig().WithRegion(region).WithCredentials(creds)

	err = createS3Folder(cfg, bucket, instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	// ravi's changes ends here
	//Finalizer - checking for instance deletion
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		// Registering finalizer to object being deleted.
		if !ContainsString(instance.ObjectMeta.Finalizers, FolderFinalizerName) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, FolderFinalizerName)
			if err := r.client.Update(context.Background(), instance); err != nil {
				log.Error(err, "Unable to add finalizer to Folder "+
					"Name", request.Name, "Namespace", request.Namespace)
				return reconcile.Result{}, err
			}
		}
	} else {
		if ContainsString(instance.ObjectMeta.Finalizers, FolderFinalizerName) {
			//deleting external dependencies
			if err := r.deleteResources(instance, cfg, bucket); err != nil {
				log.Error(err, "Unable to delete resources",
					"Name", request.Name, "Namespace", request.Namespace)
				return reconcile.Result{}, err
			}

			instance.ObjectMeta.Finalizers = RemoveString(instance.ObjectMeta.Finalizers, FolderFinalizerName)
			if err := r.client.Update(context.Background(), instance); err != nil {
				log.Error(err, "Unable to update object",
					"Name", request.Name, "Namespace", request.Namespace)
				return reconcile.Result{}, err
			}
		}
		log.Info("Reconcile Successful", "Name", request.Name,
			"Namespace", request.Namespace)
		return reconcile.Result{}, nil
	}

	// accessList := ListAwsAccessKey(cfg, aws.StringValue(createdAwsUser.UserName)).AccessKeyMetadata

	// var accessKey *iam.Accesskey
	// if len(accessList) < 1 || accessList == nil {
	// 	accessKey, err = CreateAccessKeyForUser(cfg, aws.StringValue(createdAwsUser.UserName))
	// 	if err != nil {
	// 		return reconcile.Result{}, err
	// 	}
	// } else {
	// 	awsAccessKey := accessList[0].AccessKeyId
	// 	secretExists := &corev1.Secret{}
	// 	err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.UserSecret.Name, Namespace: instance.Namespace}, secretExists)
	// 	if secretExists.Name == instance.Spec.UserSecret.Name {
	// 		if aws.StringValue(awsAccessKey) != string(secretExists.Data["aws_access_key_id"]) {
	// 			if DeleteAccessKey(cfg, aws.StringValue(awsAccessKey), aws.StringValue(createdAwsUser.UserName)) {
	// 				accessKey, err = CreateAccessKeyForUser(cfg, aws.StringValue(createdAwsUser.UserName))
	// 				if err != nil {
	// 					return reconcile.Result{}, err
	// 				}
	// 				err = r.client.Delete(context.TODO(), secretExists)
	// 				if err != nil {
	// 					return reconcile.Result{}, err
	// 				}
	// 			} else {
	// 				return reconcile.Result{}, err
	// 			}
	// 		}
	// 	} else {
	// 		if DeleteAccessKey(cfg, aws.StringValue(awsAccessKey), aws.StringValue(createdAwsUser.UserName)) {
	// 			accessKey, err = CreateAccessKeyForUser(cfg, aws.StringValue(createdAwsUser.UserName))
	// 			if err != nil {
	// 				return reconcile.Result{}, err
	// 			}
	// 			if secretExists.Name != "" {
	// 				err = r.client.Delete(context.TODO(), secretExists)
	// 				if err != nil {
	// 					return reconcile.Result{}, err
	// 				}
	// 			}
	// 		} else {
	// 			return reconcile.Result{}, err
	// 		}
	// 	}
	// }

	// secretExists := &corev1.Secret{}
	// err = r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.UserSecret.Name, Namespace: instance.Namespace}, secretExists)
	// if err != nil && errors.IsNotFound(err) {
	// 	newSecret := NewSecret(instance.Namespace, instance.Spec.UserSecret.Name, aws.StringValue(accessKey.AccessKeyId), aws.StringValue(accessKey.SecretAccessKey))
	// 	if err := controllerutil.SetControllerReference(instance, newSecret, r.scheme); err != nil {
	// 		return reconcile.Result{}, err
	// 	}

	// 	reqLogger.Info("Secret created", "secret.Namespace", newSecret.Namespace, "Secret.Name", newSecret.Name)
	// 	err = r.client.Create(context.TODO(), newSecret)
	// 	if err != nil {
	// 		return reconcile.Result{}, err
	// 	}
	// } else if err != nil {
	// 	return reconcile.Result{}, err
	// }

	return reconcile.Result{RequeueAfter: time.Second * 5}, nil

	// My changes ends
	// Define a new Pod object
	// pod := newPodForCR(instance)

	// // Set Folder instance as the owner and controller
	// if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
	// 	return reconcile.Result{}, err
	// }

	// // Check if this Pod already exists
	// found := &corev1.Pod{}
	// err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	// if err != nil && errors.IsNotFound(err) {
	// 	reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
	// 	err = r.client.Create(context.TODO(), pod)
	// 	if err != nil {
	// 		return reconcile.Result{}, err
	// 	}

	// 	// Pod created successfully - don't requeue
	// 	return reconcile.Result{}, nil
	// } else if err != nil {
	// 	return reconcile.Result{}, err
	// }

	// // Pod already exists - don't requeue
	// reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	// return reconcile.Result{}, nil
}

func NewSecret(namespace string, name string, awsAccessKey string, awsSecretKey string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"aws_access_key_id":     []byte(awsAccessKey),
			"aws_secret_access_key": []byte(awsSecretKey),
		},
	}
}

func RemoveString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func ContainsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func GetUserIdentity(cfg *aws.Config) (*sts.GetCallerIdentityOutput, error) {
	service := sts.New(session.New(), cfg)
	input := &sts.GetCallerIdentityInput{}

	result, err := service.GetCallerIdentity(input)
	if err != nil {
		if awserror, ok := err.(awserr.Error); ok {
			switch awserror.Code() {
			default:
				fmt.Println(awserror.Error())
				return nil, err
			}
		} else {
			fmt.Println(err.Error())
			return nil, err
		}
	}
	return result, nil
}

func (r *ReconcileFolder) deleteResources(instance *csye7374v1alpha1.Folder, cfg *aws.Config, bucketNm string) error {
	err := deleteS3Folder(bucketNm, instance.Spec.Username, cfg)
	if err != nil {
		return err
	}

	currentUser, err := GetUserIdentity(cfg)
	if err != nil {
		return err
	}
	accountID := aws.StringValue(currentUser.Account)

	policyArn := "arn:aws:iam::" + accountID + ":policy/" +
		instance.Spec.Username + "bucketPolicy"

	err = detachUserPolicy(policyArn, instance.Spec.Username, cfg)
	if err != nil {
		if awserror, ok := err.(awserr.Error); ok {
			if awserror.Code() != iam.ErrCodeNoSuchEntityException {
				return err
			}
		}
	}

	err = deletePolicy(policyArn, cfg)
	if err != nil {
		if awserror, ok := err.(awserr.Error); ok {
			if awserror.Code() != iam.ErrCodeNoSuchEntityException {
				return err
			}
		}
	}

	err = deleteUser(instance.Spec.Username, cfg)
	if err != nil {
		if awserror, ok := err.(awserr.Error); ok {
			if awserror.Code() != iam.ErrCodeNoSuchEntityException {
				return err
			}
		}
	}

	accessList := ListAwsAccessKey(cfg, instance.Spec.Username).AccessKeyMetadata
	if len(accessList) > 0 || accessList != nil {
		accessKeyId := accessList[0].AccessKeyId
		if !DeleteAccessKey(cfg, aws.StringValue(accessKeyId), instance.Spec.Username) {
			log.Error(err, "Could not delete Accesskey")
			return err
		}
	}
	return nil
}

func DeleteAccessKey(cfg *aws.Config, accessKey string, userName string) bool {
	service := iam.New(session.New(), cfg)
	input := &iam.DeleteAccessKeyInput{
		AccessKeyId: aws.String(accessKey),
		UserName:    aws.String(userName),
	}

	_, err := service.DeleteAccessKey(input)
	if err != nil {
		return false
	}
	return true
}

func deleteS3Folder(bucketNm string, folderNm string, cfg *aws.Config) error {
	service := s3.New(session.New(), cfg)
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucketNm),
		Key:    aws.String(folderNm + "/"),
	}

	listInput := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketNm),
		Prefix: aws.String(folderNm + "/"),
	}

	result, err := service.ListObjectsV2(listInput)
	if err != nil {
		return err
	}

	if len(result.Contents) > 0 {
		for _, obj := range result.Contents {
			objinput := &s3.DeleteObjectInput{
				Bucket: aws.String(bucketNm),
				Key:    obj.Key,
			}
			_, err = service.DeleteObject(objinput)
			if err != nil {
				log.Error(err, "Unable to delete objects inside folder %s", aws.StringValue(obj.Key))
				return err
			}
		}
	}

	_, err = service.DeleteObject(input)
	if err != nil {
		if awserror, ok := err.(awserr.Error); ok {
			switch awserror.Code() {
			default:
				return err
			}
		} else {
			return err
		}
		return err
	}
	return nil
}

func ListAwsAccessKey(cfg *aws.Config, userName string) *iam.ListAccessKeysOutput {
	service := iam.New(session.New(), cfg)
	input := &iam.ListAccessKeysInput{
		UserName: aws.String(userName),
	}
	result, err := service.ListAccessKeys(input)
	if err != nil {
		return nil
	}
	return result
}

func deleteUser(userName string, cfg *aws.Config) error {
	service := iam.New(session.New(), cfg)
	input := &iam.DeleteUserInput{
		UserName: aws.String(userName),
	}

	_, err := service.DeleteUser(input)
	if err != nil {
		return err
	}
	return nil
}
func deletePolicy(arn string, cfg *aws.Config) error {
	service := iam.New(session.New(), cfg)
	input := &iam.DeletePolicyInput{
		PolicyArn: aws.String(arn),
	}

	_, err := service.DeletePolicy(input)
	if err != nil {
		return err
	}
	return nil
}

func detachUserPolicy(arn string, userName string, cfg *aws.Config) error {
	service := iam.New(session.New(), cfg)
	input := &iam.DetachUserPolicyInput{
		PolicyArn: aws.String(arn),
		UserName:  aws.String(userName),
	}

	_, err := service.DetachUserPolicy(input)
	if err != nil {
		return err
	}

	return nil
}

func createS3Folder(cfg *aws.Config, bucket string, instance *csye7374v1alpha1.Folder) error {
	s3Service := s3.New(session.New(), cfg)
	input := &s3.GetBucketLocationInput{
		Bucket: aws.String(bucket),
	}

	bucketexists := true

	result, err := s3Service.GetBucketLocation(input)
	if awserr, ok := err.(awserr.Error); ok && awserr.Code() == s3.ErrCodeNoSuchBucket {
		bucketexists = false
		return err
	}
	fmt.Print(result)

	if bucketexists {
		key := instance.Spec.Username + "/"
		_, err := s3Service.PutObject(&s3.PutObjectInput{
			Body:   strings.NewReader("Hello World!"),
			Bucket: &bucket,
			Key:    &key,
		})
		if err != nil {
			return err
		}

	}
	if !bucketexists {
		log.Error(err, "Bucket does not exist")
		return err
	}
	return nil
}

// // newPodForCR returns a busybox pod with the same name/namespace as the cr
// func newPodForCR(cr *csye7374v1alpha1.Folder) *corev1.Pod {
// 	labels := map[string]string{
// 		"app": cr.Name,
// 	}
// 	return &corev1.Pod{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      cr.Name + "-pod",
// 			Namespace: cr.Namespace,
// 			Labels:    labels,
// 		},
// 		Spec: corev1.PodSpec{
// 			Containers: []corev1.Container{
// 				{
// 					Name:    "busybox",
// 					Image:   "busybox",
// 					Command: []string{"sleep", "3600"},
// 				},
// 			},
// 		},
// 	}
// }
