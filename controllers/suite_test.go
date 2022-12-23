// Copyright 2022.
// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Open Component Model contributors.
//
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/fluxcd/pkg/apis/meta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
)

type testEnv struct {
	scheme *runtime.Scheme
	obj    []client.Object
}

// FakeKubeClientOption defines options to construct a fake kube client. There are some defaults involved.
// Scheme gets corev1 and v1alpha1 schemes by default. Anything that is passed in will override current
// defaults.
type FakeKubeClientOption func(testEnv *testEnv)

// WithScheme provides an option to set the scheme.
func WithScheme(scheme *runtime.Scheme) FakeKubeClientOption {
	return func(testEnv *testEnv) {
		testEnv.scheme = scheme
	}
}

// WithObjects provides an option to set objects for the fake client.
func WithObjets(obj ...client.Object) FakeKubeClientOption {
	return func(testEnv *testEnv) {
		testEnv.obj = obj
	}
}

// FakeKubeClient creates a fake kube client with some defaults and optional arguments.
func (t *testEnv) FakeKubeClient(opts ...FakeKubeClientOption) client.Client {
	for _, o := range opts {
		o(t)
	}
	return fake.NewClientBuilder().WithScheme(t.scheme).WithObjects(t.obj...).Build()
}

var (
	DefaultComponent = &v1alpha1.ComponentVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-component",
			Namespace: "default",
		},
		Spec: v1alpha1.ComponentVersionSpec{
			Interval:  metav1.Duration{Duration: 10 * time.Minute},
			Component: "github.com/open-component-model/test-component",
			Version: v1alpha1.Version{
				Semver: "v0.0.1",
			},
			Repository: v1alpha1.Repository{
				URL: "github.com/open-component-model/test",
			},
			Verify: []v1alpha1.Signature{},
			References: v1alpha1.ReferencesConfig{
				Expand: true,
			},
		},
	}
	DefaultResource = &v1alpha1.Resource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-resource",
			Namespace: "default",
		},
		Spec: v1alpha1.ResourceSpec{
			Interval: metav1.Duration{Duration: 10 * time.Minute},
			ComponentVersionRef: meta.NamespacedObjectReference{
				Name:      "test-component",
				Namespace: "default",
			},
			Resource: v1alpha1.ResourceRef{
				Name:    "test-resource",
				Version: "v0.0.1",
				ReferencePath: []map[string]string{
					{
						"name": "test",
					},
				},
			},
			SnapshotTemplate: v1alpha1.SnapshotTemplateSpec{
				Name: "snapshot-test-name",
				Tag:  "v0.0.1",
			},
		},
	}
)

type CreateComponentVersionOption struct {
	componentOverride []byte
}

type CreateComponentVersionOptionFunc func(opts *CreateComponentVersionOption)

func WithComponentVersionPatch(patch []byte) CreateComponentVersionOptionFunc {
	return func(opts *CreateComponentVersionOption) {
		opts.componentOverride = patch
	}
}

func (t *testEnv) CreateComponentVersion(opts ...CreateComponentVersionOptionFunc) (*v1alpha1.ComponentVersion, error) {
	component := DefaultComponent
	defaultOpts := &CreateComponentVersionOption{}
	for _, opt := range opts {
		opt(defaultOpts)
	}
	if defaultOpts.componentOverride == nil {
		return component, nil
	}

	return mergeObjects(component, defaultOpts.componentOverride)
}

type CreateResourceOption struct {
	resourceOverride []byte
}

type CreateResourceOptionFunc func(opts *CreateResourceOption)

func WithCreateResourcePatch(r []byte) CreateResourceOptionFunc {
	return func(opts *CreateResourceOption) {
		opts.resourceOverride = r
	}
}

func (t *testEnv) CreateResource(opts ...CreateResourceOptionFunc) (*v1alpha1.Resource, error) {
	resource := DefaultResource
	defaultOpts := &CreateResourceOption{}
	for _, opt := range opts {
		opt(defaultOpts)
	}
	if defaultOpts.resourceOverride == nil {
		return resource, nil
	}

	return mergeObjects(resource, defaultOpts.resourceOverride)
}

func mergeObjects[T runtime.Object](defaultObj T, override []byte) (T, error) {
	var result T

	originalObjJS, err := runtime.Encode(unstructured.UnstructuredJSONScheme, defaultObj)
	if err != nil {
		return result, err
	}

	patchBytes, err := yaml.ToJSON(override)
	if err != nil {
		return result, fmt.Errorf("unable to parse: %w", err)
	}
	patch, err := strategicpatch.CreateTwoWayMergePatch(originalObjJS, patchBytes, defaultObj)
	if err != nil {
		return result, fmt.Errorf("failed to create two way merge: %w", err)
	}
	merged, err := strategicpatch.StrategicMergePatch(originalObjJS, patch, defaultObj)
	if err != nil {
		return result, fmt.Errorf("failed to apply patch: %w", err)
	}
	if err := json.Unmarshal(merged, defaultObj); err != nil {
		return result, fmt.Errorf("failed to create unstructured: %w", err)
	}

	return defaultObj, nil
}

var env *testEnv

func TestMain(m *testing.M) {
	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	env = &testEnv{
		scheme: scheme,
	}
	m.Run()
}
