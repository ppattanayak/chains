/*
Copyright 2022 The Tekton Authors
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

package taskrun

import (
	"context"
	"fmt"
	"reflect"

	intoto "github.com/in-toto/in-toto-golang/in_toto"
	slsa "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	"github.com/tektoncd/chains/pkg/chains/formats/slsa/extract"
	"github.com/tektoncd/chains/pkg/chains/formats/slsa/internal/material"
	slsav1 "github.com/tektoncd/chains/pkg/chains/formats/slsa/v1/taskrun"
	"github.com/tektoncd/chains/pkg/chains/objects"
	"github.com/tektoncd/chains/pkg/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/logging"
)

// BuildConfig is the custom Chains format to fill out the
// "buildConfig" section of the slsa-provenance predicate
type BuildConfig struct {
	TaskSpec       *v1beta1.TaskSpec       `json:"taskSpec"`
	TaskRunResults []v1beta1.TaskRunResult `json:"taskRunResults"`
}

func GenerateAttestation(builderID string, payloadType config.PayloadType, tro *objects.TaskRunObject, ctx context.Context) (interface{}, error) {
	logger := logging.FromContext(ctx)
	subjects := extract.SubjectDigests(tro, logger)

	mat, err := material.Materials(tro, logger)
	if err != nil {
		return nil, err
	}
	att := intoto.ProvenanceStatement{
		StatementHeader: intoto.StatementHeader{
			Type:          intoto.StatementInTotoV01,
			PredicateType: slsa.PredicateSLSAProvenance,
			Subject:       subjects,
		},
		Predicate: slsa.ProvenancePredicate{
			Builder: slsa.ProvenanceBuilder{
				ID: builderID,
			},
			BuildType:   fmt.Sprintf("https://chains.tekton.dev/format/%v/type/%s", payloadType, tro.GetGVK()),
			Invocation:  invocation(tro),
			BuildConfig: BuildConfig{TaskSpec: tro.Status.TaskSpec, TaskRunResults: tro.Status.TaskRunResults},
			Metadata:    slsav1.Metadata(tro),
			Materials:   mat,
		},
	}
	return att, nil
}

// invocation describes the event that kicked off the build
// we currently don't set ConfigSource because we don't know
// which material the Task definition came from
func invocation(tro *objects.TaskRunObject) slsa.ProvenanceInvocation {
	i := slsa.ProvenanceInvocation{}
	if p := tro.Status.Provenance; p != nil {
		i.ConfigSource = slsa.ConfigSource{
			URI:        p.ConfigSource.URI,
			Digest:     p.ConfigSource.Digest,
			EntryPoint: p.ConfigSource.EntryPoint,
		}
	}
	i.Parameters = invocationParams(tro)
	return i
}

// invocationParams adds all fields from the task run object except
// TaskRef or TaskSpec since they are in the ConfigSource or buildConfig.
func invocationParams(tro *objects.TaskRunObject) map[string]any {
	var iParams map[string]any = make(map[string]any)
	skipFields := sets.NewString("TaskRef", "TaskSpec")
	v := reflect.ValueOf(tro.Spec)
	for i := 0; i < v.NumField(); i++ {
		fieldName := v.Type().Field(i).Name
		if !skipFields.Has(v.Type().Field(i).Name) {
			iParams[fieldName] = v.Field(i).Interface()
		}
	}
	return iParams
}