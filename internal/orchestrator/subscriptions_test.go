// Copyright © 2021 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package orchestrator

import (
	"context"
	"crypto/tls"
	"fmt"
	"testing"

	"github.com/hyperledger/firefly-common/pkg/fftypes"
	"github.com/hyperledger/firefly/internal/events/system"
	"github.com/hyperledger/firefly/internal/events/webhooks"
	"github.com/hyperledger/firefly/mocks/eventmocks"
	"github.com/hyperledger/firefly/pkg/core"
	"github.com/hyperledger/firefly/pkg/database"
	"github.com/hyperledger/firefly/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateSubscriptionBadName(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	_, err := or.CreateSubscription(or.ctx, &core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name: "!sub1",
		},
	})
	assert.Regexp(t, "FF00140", err)
}

func TestCreateSubscriptionSystemTransport(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	_, err := or.CreateSubscription(or.ctx, &core.Subscription{
		Transport: system.SystemEventsTransport,
		SubscriptionRef: core.SubscriptionRef{
			Name: "sub1",
		},
	})
	assert.Regexp(t, "FF10266", err)
}

func TestCreateSubscriptionBadBatchTimeout(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	badTimeout := "-abc"
	_, err := or.CreateSubscription(or.ctx, &core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name: "sub1",
		},
		Options: core.SubscriptionOptions{
			SubscriptionCoreOptions: core.SubscriptionCoreOptions{
				BatchTimeout: &badTimeout,
			},
			WebhookSubOptions: core.WebhookSubOptions{
				URL: "http://example.com",
			},
		},
		Transport: "webhooks",
	})
	assert.Regexp(t, "FF00137", err)
}

func TestCreateSubscriptionBatchNotSupported(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	truthy := true
	_, err := or.CreateSubscription(or.ctx, &core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name: "sub1",
		},
		Options: core.SubscriptionOptions{
			SubscriptionCoreOptions: core.SubscriptionCoreOptions{
				Batch: &truthy,
			},
			WebhookSubOptions: core.WebhookSubOptions{
				URL: "http://example.com",
			},
		},
		Transport: "webhooks",
	})
	assert.Regexp(t, "FF10461", err)
}

func TestCreateSubscriptionBatchWithData(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	truthy := true
	_, err := or.CreateSubscription(or.ctx, &core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name: "sub1",
		},
		Options: core.SubscriptionOptions{
			SubscriptionCoreOptions: core.SubscriptionCoreOptions{
				WithData: &truthy,
				Batch:    &truthy,
			},
			WebhookSubOptions: core.WebhookSubOptions{
				URL: "http://example.com",
			},
		},
		Transport: "webhooks",
	})
	assert.Regexp(t, "FF10460", err)
}

func TestCreateSubscriptionBadTransport(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	or.mem = &eventmocks.EventManager{}
	or.mem.On("ResolveTransportAndCapabilities", mock.Anything, "wrongun").Return("", nil, fmt.Errorf("not found"))
	or.events = or.mem
	_, err := or.CreateSubscription(or.ctx, &core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name: "sub1",
		},
		Transport: "wrongun",
	})
	assert.Regexp(t, "not found", err)
}

func TestCreateSubscriptionOk(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	sub := &core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name: "sub1",
		},
	}
	or.mem.On("CreateUpdateDurableSubscription", mock.Anything, mock.Anything, true).Return(nil)
	s1, err := or.CreateSubscription(or.ctx, sub)
	assert.NoError(t, err)
	assert.Equal(t, s1, sub)
	assert.Equal(t, "ns", sub.Namespace)
}

func TestCreateSubscriptionTLSConfigOk(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	mockTlSConfig := &tls.Config{}

	or.namespace.TLSConfigs = map[string]*tls.Config{
		"myconfig": mockTlSConfig,
	}

	sub := &core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name: "sub1",
		},
		Options: core.SubscriptionOptions{
			WebhookSubOptions: core.WebhookSubOptions{
				TLSConfigName: "myconfig",
			},
		},
		Transport: "webhooks",
	}

	or.mem.On("CreateUpdateDurableSubscription", mock.Anything, mock.Anything, true).Return(nil)
	s1, err := or.CreateSubscription(or.ctx, sub)
	assert.NoError(t, err)
	assert.Equal(t, s1, sub)
	assert.Equal(t, "ns", sub.Namespace)
	assert.Equal(t, mockTlSConfig, s1.Options.TLSConfig)
}

func TestCreateSubscriptionTLSConfigNotFound(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	or.plugins.Events = map[string]events.Plugin{
		"webhooks": &webhooks.WebHooks{},
	}

	sub := &core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name: "sub1",
		},
		Options: core.SubscriptionOptions{
			WebhookSubOptions: core.WebhookSubOptions{
				TLSConfigName: "myconfig",
			},
		},
		Transport: "webhooks",
	}
	_, err := or.CreateSubscription(or.ctx, sub)
	assert.Error(t, err)
	assert.Regexp(t, "FF10455", err)
}

func TestCreateUpdateSubscriptionOk(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	sub := &core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			Name: "sub1",
		},
	}
	or.mem.On("CreateUpdateDurableSubscription", mock.Anything, mock.Anything, false).Return(nil)
	s1, err := or.CreateUpdateSubscription(or.ctx, sub)
	assert.NoError(t, err)
	assert.Equal(t, s1, sub)
	assert.Equal(t, "ns", sub.Namespace)
}
func TestDeleteSubscriptionBadUUID(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	err := or.DeleteSubscription(or.ctx, "! a UUID")
	assert.Regexp(t, "FF00138", err)
}

func TestDeleteSubscriptionLookupError(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	or.mdi.On("GetSubscriptionByID", mock.Anything, "ns", mock.Anything).Return(nil, fmt.Errorf("pop"))
	err := or.DeleteSubscription(or.ctx, fftypes.NewUUID().String())
	assert.EqualError(t, err, "pop")
}

func TestDeleteSubscriptionNotFound(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	or.mdi.On("GetSubscriptionByID", mock.Anything, "ns", mock.Anything).Return(nil, nil)
	err := or.DeleteSubscription(or.ctx, fftypes.NewUUID().String())
	assert.Regexp(t, "FF10109", err)
}

func TestDeleteSubscription(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	sub := &core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			ID:        fftypes.NewUUID(),
			Name:      "sub1",
			Namespace: "ns1",
		},
	}
	or.mdi.On("GetSubscriptionByID", mock.Anything, "ns", sub.ID).Return(sub, nil)
	or.mem.On("DeleteDurableSubscription", mock.Anything, sub).Return(nil)
	err := or.DeleteSubscription(or.ctx, sub.ID.String())
	assert.NoError(t, err)
}

func TestGetSubscriptions(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	u := fftypes.NewUUID()
	or.mdi.On("GetSubscriptions", mock.Anything, "ns", mock.Anything).Return([]*core.Subscription{}, nil, nil)
	fb := database.SubscriptionQueryFactory.NewFilter(context.Background())
	f := fb.And(fb.Eq("id", u))
	_, _, err := or.GetSubscriptions(context.Background(), f)
	assert.NoError(t, err)
}

func TestGetSGetSubscriptionsByID(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	u := fftypes.NewUUID()
	or.mdi.On("GetSubscriptionByID", mock.Anything, "ns", u).Return(nil, nil)
	_, err := or.GetSubscriptionByID(context.Background(), u.String())
	assert.NoError(t, err)
}

func TestGetSubscriptionDefsByIDBadID(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	_, err := or.GetSubscriptionByID(context.Background(), "")
	assert.Regexp(t, "FF00138", err)
}

func TestGetSGetSubscriptionsByIDWithStatus(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	u := fftypes.NewUUID()
	sub := &core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			ID:        u,
			Name:      "sub1",
			Namespace: "ns1",
		},
	}
	or.mdi.On("GetSubscriptionByID", context.Background(), "ns", u).Return(sub, nil)
	or.mdi.On("GetOffset", context.Background(), core.OffsetTypeSubscription, u.String()).Return(&core.Offset{Current: 100}, nil)
	subWithStatus, err := or.GetSubscriptionByIDWithStatus(context.Background(), u.String())
	assert.NoError(t, err)
	assert.NotNil(t, subWithStatus)
}

func TestGetSGetSubscriptionsByIDWithStatusQuerySubFail(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	u := fftypes.NewUUID()
	or.mdi.On("GetSubscriptionByID", context.Background(), "ns", u).Return(nil, fmt.Errorf("pop"))
	subWithStatus, err := or.GetSubscriptionByIDWithStatus(context.Background(), u.String())
	assert.EqualError(t, err, "pop")
	assert.Nil(t, subWithStatus)
}

func TestGetSGetSubscriptionsByIDWithStatusOffsetQueryError(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	u := fftypes.NewUUID()
	sub := &core.Subscription{
		SubscriptionRef: core.SubscriptionRef{
			ID:        u,
			Name:      "sub1",
			Namespace: "ns1",
		},
	}
	or.mdi.On("GetSubscriptionByID", context.Background(), "ns", u).Return(sub, nil)
	or.mdi.On("GetOffset", context.Background(), core.OffsetTypeSubscription, u.String()).Return(nil, fmt.Errorf("pop"))
	subWithStatus, err := or.GetSubscriptionByIDWithStatus(context.Background(), u.String())
	assert.EqualError(t, err, "pop")
	assert.Nil(t, subWithStatus)
}

func TestGetSGetSubscriptionsByIDWithStatusUnknownSub(t *testing.T) {
	or := newTestOrchestrator()
	defer or.cleanup(t)

	u := fftypes.NewUUID()
	or.mdi.On("GetSubscriptionByID", context.Background(), "ns", u).Return(nil, nil)
	subWithStatus, err := or.GetSubscriptionByIDWithStatus(context.Background(), u.String())
	assert.NoError(t, err)
	assert.Nil(t, subWithStatus)
}
