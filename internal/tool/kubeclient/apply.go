package kubeclient

import (
	"bytes"
	"context"
	"fmt"
	"github.com/jonboulle/clockwork"
	"io"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	oapi "k8s.io/kube-openapi/pkg/util/proto"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/openapi"
	"os"
	"time"
)

const (
	// maxPatchRetry is the maximum number of conflicts retry
	// for during a patch operation before returning failure
	maxPatchRetry = 5

	// how many times we can retry before back off
	triesBeforeBackOff = 1

	// backOffPeriod is the period to back off when apply patch resutls in error.
	backOffPeriod = 1 * time.Second
)

func BuildClientGetter(
	kubeconfigPath string,
) genericclioptions.RESTClientGetter {

	if kubeconfigPath == "" {
		return NewClientGetterInCluster()
	}
	loadingRules := clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	apicfg, err := loadingRules.Load()
	if err != nil {
		klog.Errorf("load kubeconfig file: %s, fallback to in cluster config", err.Error())
	}
	// be careful, apicfg == nil stand for in cluster config
	return NewClientGetter(apicfg)
}

func ApplyInCluster(yml string) error {
	return doApply(bytes.NewBufferString(yml), NewClientGetterInCluster())
}

func ApplyBy(yml string, kubeconfig string) error {
	return doApply(bytes.NewBufferString(yml), BuildClientGetter(kubeconfig))
}

func ApplyWithKubeconfig(yml, kubeconfig string) error {
	return doApply(bytes.NewBufferString(yml), BuildClientGetter(kubeconfig))
}

func DeleteInCluster(yml string) error {
	return doDelete(bytes.NewBufferString(yml), NewClientGetterInCluster())
}

func doDelete(
	reader io.Reader,
	getter genericclioptions.RESTClientGetter,
) error {
	f := cmdutil.NewFactory(getter)
	schema, err := f.Validator(metav1.FieldValidationWarn)
	if err != nil {
		return err
	}

	cmdNamespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	r := f.NewBuilder().
		Unstructured().
		Stream(reader, "Delete").
		Schema(schema).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		//FilenameParam(enforceNamespace, &options.FilenameOptions).
		LabelSelectorParam("").
		Flatten().
		Do()
	err = r.Err()
	if err != nil {
		return fmt.Errorf("object builder: %s", err.Error())
	}

	return r.Visit(
		func(info *resource.Info, err error) error {
			if err != nil {
				return fmt.Errorf("visit object: %s, info=%v", err.Error(), info)
			}
			dClient, err := f.DynamicClient()
			if err != nil {
				return err
			}
			helper := resource.NewHelper(info.Client, info.Mapping)
			patcher := &patcher{
				mapping:       info.Mapping,
				helper:        helper,
				dynamicClient: dClient,
				overwrite:     true,
				backOff:       clockwork.NewRealClock(),
				force:         true,
				cascade:       true,
				timeout:       30 * time.Second,
				gracePeriod:   30,
				openapiSchema: nil,
			}
			return patcher.delete(info.Namespace, info.Name)
		})
}

func doApply(
	reader io.Reader,
	getter genericclioptions.RESTClientGetter,
) error {
	f := cmdutil.NewFactory(getter)
	schema, err := f.Validator(metav1.FieldValidationWarn)
	if err != nil {
		return err
	}

	cmdNamespace, _, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	r := f.NewBuilder().
		Unstructured().
		Stream(reader, "Apply").
		Schema(schema).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		//FilenameParam(enforceNamespace, &options.FilenameOptions).
		LabelSelectorParam("").
		Flatten().
		Do()
	err = r.Err()
	if err != nil {
		return fmt.Errorf("object builder: %s", err.Error())
	}

	return r.Visit(
		func(info *resource.Info, err error) error {
			if err != nil {
				return fmt.Errorf("visit object: %s, info=%v", err.Error(), info)
			}
			// Get the modified configuration of the object. Embed the result
			// as an annotation in the modified configuration, so that it will appear
			// in the patch sent to the server.
			modified, err := GetModifiedConfiguration(
				info.Object, true, unstructured.UnstructuredJSONScheme,
			)
			if err != nil {
				return cmdutil.AddSourceToErr(
					fmt.Sprintf("retrieving modified configuration from:\n%v\nfor:", info),
					info.Source, err,
				)
			}

			if err := info.Get(); err != nil {
				if !errors.IsNotFound(err) {
					return cmdutil.AddSourceToErr(
						fmt.Sprintf("retrieving current configuration of:\n%v\nfrom server for:", info),
						info.Source, err,
					)
				}
				if err := createAndRefresh(info); err != nil {
					return cmdutil.AddSourceToErr("creating", info.Source, err)
				}
				return nil
			}
			dClient, err := f.DynamicClient()
			if err != nil {
				return err
			}
			helper := resource.NewHelper(info.Client, info.Mapping)
			patcher := &patcher{
				// encoder:       encoder,
				// decoder:       decoder,
				mapping:       info.Mapping,
				helper:        helper,
				dynamicClient: dClient,
				overwrite:     true,
				backOff:       clockwork.NewRealClock(),
				force:         true,
				cascade:       true,
				timeout:       30 * time.Second,
				gracePeriod:   30,
				openapiSchema: nil,
			}
			patchBytes, patchedObject, err := patcher.patch(info.Object, modified, info.Source, info.Namespace, info.Name, os.Stderr)
			if err != nil {
				return cmdutil.AddSourceToErr(
					fmt.Sprintf("applying patch:\n%s\nto:\n%v\nfor:", patchBytes, info),
					info.Source, err,
				)
			}

			return info.Refresh(patchedObject, true)
		})
}

var metadataAccessor = meta.NewAccessor()

// GetModifiedConfiguration retrieves the modified configuration of the object.
// If annotate is true, it embeds the result as an annotation in the modified
// configuration. If an object was read from the command input, it will use that
// version of the object. Otherwise, it will use the version from the server.
func GetModifiedConfiguration(obj runtime.Object, annotate bool, codec runtime.Encoder) ([]byte, error) {
	// First serialize the object without the annotation to prevent recursion,
	// then add that serialization to it as the annotation and serialize it again.
	var modified []byte

	// Otherwise, use the server side version of the object.
	// Get the current annotations from the object.
	annots, err := metadataAccessor.Annotations(obj)
	if err != nil {
		return nil, err
	}

	if annots == nil {
		annots = map[string]string{}
	}

	original := annots[v1.LastAppliedConfigAnnotation]
	delete(annots, v1.LastAppliedConfigAnnotation)
	if err := metadataAccessor.SetAnnotations(obj, annots); err != nil {
		return nil, err
	}

	modified, err = runtime.Encode(codec, obj)
	if err != nil {
		return nil, err
	}

	if annotate {
		annots[v1.LastAppliedConfigAnnotation] = string(modified)
		if err := metadataAccessor.SetAnnotations(obj, annots); err != nil {
			return nil, err
		}

		modified, err = runtime.Encode(codec, obj)
		if err != nil {
			return nil, err
		}
	}

	// Restore the object to its original check.
	annots[v1.LastAppliedConfigAnnotation] = original
	if err := metadataAccessor.SetAnnotations(obj, annots); err != nil {
		return nil, err
	}

	return modified, nil
}

// GetOriginalConfiguration retrieves the original configuration of the object
// from the annotation, or nil if no annotation was found.
func GetOriginalConfiguration(obj runtime.Object) ([]byte, error) {
	annots, err := metadataAccessor.Annotations(obj)
	if err != nil {
		return nil, err
	}

	if annots == nil {
		return nil, nil
	}

	original, ok := annots[v1.LastAppliedConfigAnnotation]
	if !ok {
		return nil, nil
	}

	return []byte(original), nil
}

// createAndRefresh creates an object from input info and refreshes info with that object
func createAndRefresh(info *resource.Info) error {
	obj, err := resource.
		NewHelper(info.Client, info.Mapping).
		Create(info.Namespace, true, info.Object)
	if err != nil {
		return err
	}
	return info.Refresh(obj, true)
}

func runDelete(namespace, name string, mapping *meta.RESTMapping, c dynamic.Interface, cascade bool, gracePeriod int) error {
	options := &metav1.DeleteOptions{}
	if gracePeriod >= 0 {
		options = metav1.NewDeleteOptions(int64(gracePeriod))
	}

	policy := metav1.DeletePropagationForeground
	if !cascade {
		policy = metav1.DeletePropagationOrphan
	}
	options.PropagationPolicy = &policy
	return c.Resource(mapping.Resource).Namespace(namespace).Delete(context.TODO(), name, *options)
}

func (p *patcher) delete(namespace, name string) error {
	return runDelete(namespace, name, p.mapping, p.dynamicClient, p.cascade, p.gracePeriod)
}

type patcher struct {
	encoder runtime.Encoder
	decoder runtime.Decoder

	mapping       *meta.RESTMapping
	helper        *resource.Helper
	dynamicClient dynamic.Interface

	overwrite bool
	backOff   clockwork.Clock

	force       bool
	cascade     bool
	timeout     time.Duration
	gracePeriod int

	openapiSchema openapi.Resources
}

func (p *patcher) patchSimple(
	obj runtime.Object,
	modified []byte,
	source, namespace, name string,
	errOut io.Writer,
) ([]byte, runtime.Object, error) {
	// Serialize the current configuration of the object from the server.
	current, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
	if err != nil {
		return nil, nil, cmdutil.AddSourceToErr(fmt.Sprintf("serializing current configuration from:\n%v\nfor:", obj), source, err)
	}

	// Retrieve the original configuration of the object from the annotation.
	original, err := GetOriginalConfiguration(obj)
	if err != nil {
		return nil, nil, cmdutil.AddSourceToErr(fmt.Sprintf("retrieving original configuration from:\n%v\nfor:", obj), source, err)
	}

	var patchType types.PatchType
	var patch []byte
	var lookupPatchMeta strategicpatch.LookupPatchMeta
	var schema oapi.Schema
	createPatchErrFormat := "creating patch with:\noriginal:\n%s\nmodified:\n%s\ncurrent:\n%s\nfor:"

	// Create the versioned struct from the type defined in the restmapping
	// (which is the API version we'll be submitting the patch to)
	versionedObject, err := scheme.Scheme.New(p.mapping.GroupVersionKind)
	switch {
	case runtime.IsNotRegisteredError(err):
		// fall back to generic JSON merge patch
		patchType = types.MergePatchType
		preconditions := []mergepatch.PreconditionFunc{mergepatch.RequireKeyUnchanged("apiVersion"),
			mergepatch.RequireKeyUnchanged("kind"), mergepatch.RequireMetadataKeyUnchanged("name")}
		patch, err = jsonmergepatch.CreateThreeWayJSONMergePatch(original, modified, current, preconditions...)
		if err != nil {
			if mergepatch.IsPreconditionFailed(err) {
				return nil, nil, fmt.Errorf("%s", "At least one of apiVersion, kind and name was changed")
			}
			return nil, nil, cmdutil.AddSourceToErr(fmt.Sprintf(createPatchErrFormat, original, modified, current), source, err)
		}
	case err != nil:
		return nil, nil, cmdutil.AddSourceToErr(fmt.Sprintf("getting instance of versioned object for %v:", p.mapping.GroupVersionKind), source, err)
	case err == nil:
		// Compute a three way strategic merge patch to send to server.
		patchType = types.StrategicMergePatchType

		// Try to use openapi first if the openapi spec is available and can successfully calculate the patch.
		// Otherwise, fall back to baked-in types.
		if p.openapiSchema != nil {
			if schema = p.openapiSchema.LookupResource(p.mapping.GroupVersionKind); schema != nil {
				lookupPatchMeta = strategicpatch.PatchMetaFromOpenAPI{Schema: schema}
				if openapiPatch, err := strategicpatch.CreateThreeWayMergePatch(original, modified, current, lookupPatchMeta, p.overwrite); err != nil {
					fmt.Fprintf(errOut, "warning: error calculating patch from openapi spec: %v\n", err)
				} else {
					patchType = types.StrategicMergePatchType
					patch = openapiPatch
				}
			}
		}

		if patch == nil {
			lookupPatchMeta, err = strategicpatch.NewPatchMetaFromStruct(versionedObject)
			if err != nil {
				return nil, nil, cmdutil.AddSourceToErr(fmt.Sprintf(createPatchErrFormat, original, modified, current), source, err)
			}
			patch, err = strategicpatch.CreateThreeWayMergePatch(original, modified, current, lookupPatchMeta, p.overwrite)
			if err != nil {
				return nil, nil, cmdutil.AddSourceToErr(fmt.Sprintf(createPatchErrFormat, original, modified, current), source, err)
			}
		}
	}

	if string(patch) == "{}" {
		return patch, obj, nil
	}

	patchedObj, err := p.helper.Patch(namespace, name, patchType, patch, &metav1.PatchOptions{})
	return patch, patchedObj, err
}

func (p *patcher) patch(
	current runtime.Object,
	modified []byte,
	source, namespace, name string,
	errOut io.Writer,
) ([]byte, runtime.Object, error) {
	var getErr error
	patchBytes, patchObject, err := p.patchSimple(current, modified, source, namespace, name, errOut)
	for i := 1; i <= maxPatchRetry && errors.IsConflict(err); i++ {
		if i > triesBeforeBackOff {
			p.backOff.Sleep(backOffPeriod)
		}
		current, getErr = p.helper.Get(namespace, name)
		if getErr != nil {
			return nil, nil, getErr
		}
		patchBytes, patchObject, err = p.patchSimple(current, modified, source, namespace, name, errOut)
	}
	if err != nil && (errors.IsConflict(err) || errors.IsInvalid(err)) && p.force {
		patchBytes, patchObject, err = p.deleteAndCreate(current, modified, namespace, name)
	}
	return patchBytes, patchObject, err
}

func (p *patcher) deleteAndCreate(
	original runtime.Object,
	modified []byte,
	namespace, name string,
) ([]byte, runtime.Object, error) {
	if err := p.delete(namespace, name); err != nil {
		return modified, nil, err
	}
	// TODO: use wait
	if err := wait.PollImmediate(1*time.Second, p.timeout, func() (bool, error) {
		if _, err := p.helper.Get(namespace, name); !errors.IsNotFound(err) {
			return false, err
		}
		return true, nil
	}); err != nil {
		return modified, nil, err
	}
	versionedObject, _, err := unstructured.UnstructuredJSONScheme.Decode(modified, nil, nil)
	if err != nil {
		return modified, nil, err
	}

	createdObject, err := p.helper.Create(namespace, true, versionedObject)
	if err != nil {
		// restore the original object if we fail to create the new one
		// but still propagate and advertise error to user
		recreated, recreateErr := p.helper.Create(namespace, true, original)
		if recreateErr != nil {
			err = fmt.Errorf("An error occurred force-replacing the existing "+
				"object with the newly provided one:\n\n%v.\n\nAdditionally, an error "+
				"occurred attempting to restore the original object:\n\n%v\n", err, recreateErr)
		} else {
			createdObject = recreated
		}
	}
	return modified, createdObject, err
}

//func applyOneObject(info *resource.Info) error {
//
//	helper := resource.NewHelper(info.Client, info.UpnpPortMapping)
//
//	// Send the full object to be applied on the server side.
//	data, err := runtime.Encode(unstructured.UnstructuredJSONScheme, info.Object)
//	if err != nil {
//		return cmdutil.AddSourceToErr("serverside-apply", info.Source, err)
//	}
//
//	options := metav1.PatchOptions{
//		Force: &o.ForceConflicts,
//	}
//	obj, err := helper.Patch(
//		info.Namespace, info.TagKey, types.ApplyPatchType, data, &options,
//	)
//	if err != nil {
//		if isIncompatibleServerError(err) {
//			err = fmt.Errorf("Server-side apply not available on the server: (%v)", err)
//		}
//		if errors.IsConflict(err) {
//			err = fmt.Errorf(`apply conflict: %v`, err)
//		}
//		return err
//	}
//
//	info.Refresh(obj, true)
//
//
//	// Get the modified configuration of the object. Embed the result
//	// as an annotation in the modified configuration, so that it will appear
//	// in the patch sent to the server.
//	modified, err := util.GetModifiedConfiguration(info.Object, true, unstructured.UnstructuredJSONScheme)
//	if err != nil {
//		return cmdutil.AddSourceToErr(fmt.Sprintf("retrieving modified configuration from:\n%s\nfor:", info.String()), info.Source, err)
//	}
//
//	if err := info.Get(); err != nil {
//		if !errors.IsNotFound(err) {
//			return cmdutil.AddSourceToErr(fmt.Sprintf("retrieving current configuration of:\n%s\nfrom server for:", info.String()), info.Source, err)
//		}
//
//		// Create the resource if it doesn't exist
//		// First, update the annotation used by kubectl apply
//		if err := util.CreateApplyAnnotation(info.Object, unstructured.UnstructuredJSONScheme); err != nil {
//			return cmdutil.AddSourceToErr("creating", info.Source, err)
//		}
//
//		// Then create the resource and skip the three-way merge
//		obj, err := helper.Create(info.Namespace, true, info.Object)
//		if err != nil {
//			return cmdutil.AddSourceToErr("creating", info.Source, err)
//		}
//		info.Refresh(obj, true)
//	}
//
//	if err := o.MarkObjectVisited(info); err != nil {
//		return err
//	}
//
//	if o.DryRunStrategy != cmdutil.DryRunClient {
//		metadata, _ := meta.Accessor(info.Object)
//		annotationMap := metadata.GetAnnotations()
//		if _, ok := annotationMap[corev1.LastAppliedConfigAnnotation]; !ok {
//			fmt.Fprintf(o.ErrOut, warningNoLastAppliedConfigAnnotation, o.cmdBaseName)
//		}
//
//		patcher, err := newPatcher(o, info, helper)
//		if err != nil {
//			return err
//		}
//		patchBytes, patchedObject, err := patcher.Patch(info.Object, modified, info.Source, info.Namespace, info.TagKey, o.ErrOut)
//		if err != nil {
//			return cmdutil.AddSourceToErr(fmt.Sprintf("applying patch:\n%s\nto:\n%v\nfor:", patchBytes, info), info.Source, err)
//		}
//
//		info.Refresh(patchedObject, true)
//
//	}
//
//	return nil
//}
//
//
//
//func newPatcher(info *resource.Info, helper *resource.Helper) (*Patcher, error) {
//	var openapiSchema openapi.Resources
//	if o.OpenAPIPatch {
//		openapiSchema = o.OpenAPISchema
//	}
//
//	return &Patcher{
//		UpnpPortMapping:       info.UpnpPortMapping,
//		Helper:        helper,
//		Overwrite:     o.Overwrite,
//		BackOff:       clockwork.NewRealClock(),
//		Force:         o.DeleteOptions.ForceDeletion,
//		Cascade:       o.DeleteOptions.Cascade,
//		Timeout:       o.DeleteOptions.Timeout,
//		GracePeriod:   o.DeleteOptions.GracePeriod,
//		OpenapiSchema: openapiSchema,
//		Retries:       maxPatchRetry,
//	}, nil
//}
