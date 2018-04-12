/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	"configcenter/src/common/blog"
	"configcenter/src/common/confregdiscover"
	"configcenter/src/common/core/cc/api"
	"configcenter/src/common/language"
	"configcenter/src/common/types"
	"context"
	"encoding/json"
	"sync"
)

// ConfCenter discover configure changed. get, update configures
type ConfCenter struct {
	confRegDiscv *confregdiscover.ConfRegDiscover
	rootCtx      context.Context
	cancel       context.CancelFunc
	ctx          []byte
	ctxLock      sync.RWMutex
	langCtx      map[string]language.LanguageMap
	langLock     sync.RWMutex
}

// NewConfCenter create a ConfCenter object
func NewConfCenter(serv string) *ConfCenter {
	return &ConfCenter{
		ctx:          nil,
		confRegDiscv: confregdiscover.NewConfRegDiscover(serv),
	}
}

// Start the configure center module service
func (cc *ConfCenter) Start() error {
	// create root context
	cc.rootCtx, cc.cancel = context.WithCancel(context.Background())

	// start configure register and discover service
	if err := cc.confRegDiscv.Start(); err != nil {
		blog.Errorf("fail to start config register and discover service. err:%s", err.Error())
		return err
	}

	// here: discover itselft configure
	confPath := types.CC_SERVCONF_BASEPATH + "/" + types.CC_MODULE_WEBSERVER
	confEvent, err := cc.confRegDiscv.DiscoverConfig(confPath)
	if err != nil {
		blog.Errorf("fail to discover configure for migrate service. err:%s", err.Error())
		return err
	}

	langEvent, err := cc.confRegDiscv.DiscoverConfig(types.CC_SERVLANG_BASEPATH)
	if err != nil {
		blog.Errorf("fail to discover configure for migrate service. err:%s", err.Error())
		return err
	}

	for {
		select {
		case confEvn := <-confEvent:
			cc.dealConfChangeEvent(confEvn.Data)
		case langEvn := <-langEvent:
			cc.dealLanguageResEvent(langEvn.Data)
		case <-cc.rootCtx.Done():
			blog.Warn("configure discover service done")
			return nil
		}
	}
}

// Stop the configure center
func (cc *ConfCenter) Stop() error {
	cc.cancel()

	cc.confRegDiscv.Stop()

	return nil
}

// GetConfigureCtx fetch the configure
func (cc *ConfCenter) GetConfigureCtx() []byte {
	cc.ctxLock.RLock()
	defer cc.ctxLock.RUnlock()

	return cc.ctx
}

func (cc *ConfCenter) dealConfChangeEvent(data []byte) error {
	blog.Info("%s configure has changed", types.CC_MODULE_WEBSERVER)

	cc.ctxLock.Lock()
	defer cc.ctxLock.Unlock()

	cc.ctx = data

	return nil
}

// GetLanguageResCxt fetch the language packages
func (cc *ConfCenter) GetLanguageResCxt() map[string]language.LanguageMap {
	cc.langLock.RLock()
	defer cc.langLock.RUnlock()

	return cc.langCtx
}

func (cc *ConfCenter) dealLanguageResEvent(data []byte) error {
	blog.Info("language has changed")

	cc.langLock.Lock()
	defer cc.langLock.Unlock()

	langMap := map[string]language.LanguageMap{}
	err := json.Unmarshal(data, &langMap)
	if err != nil {
		return err
	}

	cc.langCtx = langMap
	a := api.GetAPIResource()
	if a.Lang != nil {
		a.Lang.Load(langMap)
	}

	return nil
}
