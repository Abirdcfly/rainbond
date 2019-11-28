// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/go-chi/chi"
	"github.com/goodrain/rainbond/api/handler"
	"github.com/goodrain/rainbond/api/middleware"
	api_model "github.com/goodrain/rainbond/api/model"
	"github.com/goodrain/rainbond/api/util"
	dbmodel "github.com/goodrain/rainbond/db/model"
	httputil "github.com/goodrain/rainbond/util/http"
	"github.com/jinzhu/gorm"
)

// GetVolumesStatus getvolume status
func (t *TenantStruct) GetVolumesStatus(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/volumes-status v2 GetVolumesStatus
	//
	// 查询组件存储状态
	//
	// post volumes-status
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//     description: 统一返回格式
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	volumes, handlerErr := handler.GetServiceManager().GetVolumes(serviceID)
	if handlerErr != nil && handlerErr.Error() != gorm.ErrRecordNotFound.Error() {
		httputil.ReturnError(r, w, 500, handlerErr.Error())
		return
	}
	var err error
	volumeStatusList, err := t.StatusCli.GetAppVolumeStatus(serviceID)
	if err != nil {
		logrus.Warnf("get volume status error: %s", err.Error())
	}
	ret := api_model.VolumeWithStatusResp{Status: make(map[string]string)}
	if volumeStatusList != nil && volumeStatusList.GetStatus() != nil {
		volumeStatus := volumeStatusList.GetStatus()
		status := make(map[string]string)
		for _, volume := range volumes {
			if phrase, ok := volumeStatus[volume.VolumeName]; ok {
				status[volume.VolumeName] = phrase.String()
			}
		}
		ret.Status = status
	}
	httputil.ReturnSuccess(r, w, ret)
}

// VolumeBestSelector best volume by volume filter
func (t *TenantStruct) VolumeBestSelector(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/volume-best v2 VolumeBest
	//
	// 查询可用存储驱动模型列表
	//
	// post volume-best
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//     description: 统一返回格式
	var oldVolumeSelector api_model.VolumeBestReqStruct
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &oldVolumeSelector, nil); !ok {
		return
	}
	if oldVolumeSelector.VolumeType == dbmodel.ShareFileVolumeType.String() || oldVolumeSelector.VolumeType == dbmodel.LocalVolumeType.String() {
		ret := api_model.VolumeBestRespStruct{Changed: false}
		httputil.ReturnSuccess(r, w, ret)
		return
	}
	storageClasses, err := t.StatusCli.GetStorageClasses()
	if err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	var providerMap = make(map[string][]api_model.VolumeProviderDetail)
	kindFilter := oldVolumeSelector.VolumeType
	for _, storageClass := range storageClasses.GetList() {
		kind := util.ParseVolumeProviderKind(storageClass)
		if kind == "" {
			logrus.Debugf("unknown storageclass: %+v", storageClass)
			continue
		}
		if kindFilter != "" && kind != kindFilter {
			continue
		}
		detail := api_model.VolumeProviderDetail{
			Name:                 storageClass.Name,
			Provisioner:          storageClass.Provisioner,
			ReclaimPolicy:        storageClass.ReclaimPolicy,
			VolumeBindingMode:    storageClass.VolumeBindingMode,
			AllowVolumeExpansion: &storageClass.AllowVolumeExpansion,
		}
		util.HackVolumeProviderDetail(kind, &detail)
		exists := false
		for _, accessMode := range detail.AccessMode {
			if strings.ToUpper(oldVolumeSelector.AccessMode) == accessMode {
				exists = true
				break
			}
		}
		if !exists {
			logrus.Warnf("not suitable select for volume[volumeType:%s, accessMode:%s] of kind(%s)", oldVolumeSelector.VolumeType, oldVolumeSelector.AccessMode, kind)
			continue
		} else {
			providerMap[kind] = []api_model.VolumeProviderDetail{detail}
			break
		}

	}
	ret := api_model.VolumeBestRespStruct{}
	if len(providerMap) > 0 {
		ret.Changed = false
	} else {
		ret.Changed = true
		ret.VolumeType = dbmodel.ShareFileVolumeType.String()
	}

	httputil.ReturnSuccess(r, w, ret)
}

// VolumeProvider list volume provider
func (t *TenantStruct) VolumeProvider(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/volume-providers v2 volumeProvider
	//
	// 查询可用存储驱动模型列表
	//
	// get volume-providers
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//     description: 统一返回格式

	kindFilter := r.FormValue("kind")
	storageClasses, err := t.StatusCli.GetStorageClasses()
	if err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
		return
	}
	var providerList []api_model.VolumeProviderStruct
	var providerMap = make(map[string][]api_model.VolumeProviderDetail)

	for _, storageClass := range storageClasses.GetList() {
		kind := util.ParseVolumeProviderKind(storageClass)
		if kind == "" {
			logrus.Debugf("not support storageclass: %+v", storageClass)
			continue
		}
		if kindFilter != "" && kind != kindFilter {
			continue
		}
		detail := api_model.VolumeProviderDetail{
			Name:                 storageClass.Name,
			Provisioner:          storageClass.Provisioner,
			ReclaimPolicy:        storageClass.ReclaimPolicy,
			VolumeBindingMode:    storageClass.VolumeBindingMode,
			AllowVolumeExpansion: &storageClass.AllowVolumeExpansion,
		}
		util.HackVolumeProviderDetail(kind, &detail)
		if _, ok := providerMap[kind]; ok {
			providerMap[kind] = append(providerMap[kind], detail)
		} else {
			providerMap[kind] = []api_model.VolumeProviderDetail{detail}
		}
	}
	for key, value := range providerMap {
		providerList = append(providerList, api_model.VolumeProviderStruct{Kind: key, Provisioner: value})
	}
	httputil.ReturnSuccess(r, w, providerList)
}

//VolumeDependency VolumeDependency
func (t *TenantStruct) VolumeDependency(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "DELETE":
		t.DeleteVolumeDependency(w, r)
	case "POST":
		t.AddVolumeDependency(w, r)
	}
}

//AddVolumeDependency add volume dependency
func (t *TenantStruct) AddVolumeDependency(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/volume-dependency v2 addVolumeDependency
	//
	// 增加应用持久化依赖
	//
	// add volume dependency
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	logrus.Debugf("trans add volumn dependency service ")
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	var tsr api_model.V2AddVolumeDependencyStruct
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &tsr.Body, nil); !ok {
		return
	}
	vd := &dbmodel.TenantServiceMountRelation{
		TenantID:        tenantID,
		ServiceID:       serviceID,
		DependServiceID: tsr.Body.DependServiceID,
		HostPath:        tsr.Body.MntDir,
		VolumePath:      tsr.Body.MntName,
	}
	if err := handler.GetServiceManager().VolumeDependency(vd, "add"); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//DeleteVolumeDependency delete volume dependency
func (t *TenantStruct) DeleteVolumeDependency(w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /v2/tenants/{tenant_name}/services/{service_alias}/volume-dependency v2 deleteVolumeDependency
	//
	// 删除应用持久化依赖
	//
	// delete volume dependency
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	var tsr api_model.V2DelVolumeDependencyStruct
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &tsr.Body, nil); !ok {
		return
	}
	vd := &dbmodel.TenantServiceMountRelation{
		TenantID:        tenantID,
		ServiceID:       serviceID,
		DependServiceID: tsr.Body.DependServiceID,
	}
	if err := handler.GetServiceManager().VolumeDependency(vd, "delete"); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//AddVolume AddVolume
func (t *TenantStruct) AddVolume(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/volume v2 addVolume
	//
	// 增加应用持久化信息
	//
	// add volume
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	avs := &api_model.V2AddVolumeStruct{}
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &avs.Body, nil); !ok {
		return
	}
	tsv := &dbmodel.TenantServiceVolume{
		ServiceID:          serviceID,
		VolumePath:         avs.Body.VolumePath,
		HostPath:           avs.Body.HostPath,
		Category:           avs.Body.Category,
		VolumeCapacity:     avs.Body.VolumeCapacity,
		VolumeType:         dbmodel.ShareFileVolumeType.String(),
		VolumeProviderName: avs.Body.VolumeProviderName,
		AccessMode:         avs.Body.AccessMode,
		SharePolicy:        avs.Body.SharePolicy,
		BackupPolicy:       avs.Body.BackupPolicy,
		ReclaimPolicy:      avs.Body.ReclaimPolicy,
	}
	if !strings.HasPrefix(tsv.VolumePath, "/") {
		httputil.ReturnError(r, w, 400, "volume path is invalid,must begin with /")
		return
	}
	if err := handler.GetServiceManager().VolumnVar(tsv, tenantID, "", "add"); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

// UpdVolume updates service volume.
func (t *TenantStruct) UpdVolume(w http.ResponseWriter, r *http.Request) {
	var req api_model.UpdVolumeReq
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &req, nil); !ok {
		return
	}

	sid := r.Context().Value(middleware.ContextKey("service_id")).(string)
	if err := handler.GetServiceManager().UpdVolume(sid, &req); err != nil {
		httputil.ReturnError(r, w, 500, err.Error())
	}
	httputil.ReturnSuccess(r, w, "success")
}

//DeleteVolume DeleteVolume
func (t *TenantStruct) DeleteVolume(w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /v2/tenants/{tenant_name}/services/{service_alias}/volume v2 deleteVolume
	//
	// 删除应用持久化信息
	//
	// delete volume
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	avs := &api_model.V2DelVolumeStruct{}
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &avs.Body, nil); !ok {
		return
	}
	tsv := &dbmodel.TenantServiceVolume{
		ServiceID:  serviceID,
		VolumePath: avs.Body.VolumePath,
		Category:   avs.Body.Category,
	}
	if err := handler.GetServiceManager().VolumnVar(tsv, tenantID, "", "delete"); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//以下为V2.1版本持久化API,支持多种持久化模式

//AddVolumeDependency add volume dependency
func AddVolumeDependency(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/depvolumes v2 addDepVolume
	//
	// 增加应用持久化依赖(V2.1支持多种类型存储)
	//
	// add volume dependency
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	logrus.Debugf("trans add volumn dependency service ")
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	var tsr api_model.AddVolumeDependencyStruct
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &tsr.Body, nil); !ok {
		return
	}

	vd := &dbmodel.TenantServiceMountRelation{
		TenantID:        tenantID,
		ServiceID:       serviceID,
		DependServiceID: tsr.Body.DependServiceID,
		VolumeName:      tsr.Body.VolumeName,
		VolumePath:      tsr.Body.VolumePath,
		VolumeType:      tsr.Body.VolumeType,
	}
	if err := handler.GetServiceManager().VolumeDependency(vd, "add"); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//DeleteVolumeDependency delete volume dependency
func DeleteVolumeDependency(w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /v2/tenants/{tenant_name}/services/{service_alias}/depvolumes v2 delDepVolume
	//
	// 删除应用持久化依赖(V2.1支持多种类型存储)
	//
	// delete volume dependency
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	var tsr api_model.DeleteVolumeDependencyStruct
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &tsr.Body, nil); !ok {
		return
	}
	vd := &dbmodel.TenantServiceMountRelation{
		TenantID:        tenantID,
		ServiceID:       serviceID,
		DependServiceID: tsr.Body.DependServiceID,
		VolumeName:      tsr.Body.VolumeName,
	}
	if err := handler.GetServiceManager().VolumeDependency(vd, "delete"); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//AddVolume AddVolume
func AddVolume(w http.ResponseWriter, r *http.Request) {
	// swagger:operation POST /v2/tenants/{tenant_name}/services/{service_alias}/volumes v2 addVolumes
	//
	// 增加应用持久化信息(V2.1支持多种类型存储)
	//
	// add volume
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	avs := &api_model.AddVolumeStruct{}
	if ok := httputil.ValidatorRequestStructAndErrorResponse(r, w, &avs.Body, nil); !ok {
		return
	}
	bytes, _ := json.Marshal(avs)
	logrus.Debugf("request uri: %s; request body: %v", r.RequestURI, string(bytes))

	tsv := &dbmodel.TenantServiceVolume{
		ServiceID:          serviceID,
		VolumeName:         avs.Body.VolumeName,
		VolumePath:         avs.Body.VolumePath,
		VolumeType:         avs.Body.VolumeType,
		Category:           avs.Body.Category,
		VolumeProviderName: avs.Body.VolumeProviderName,
		IsReadOnly:         avs.Body.IsReadOnly,
		VolumeCapacity:     avs.Body.VolumeCapacity,
		AccessMode:         avs.Body.AccessMode,
		SharePolicy:        avs.Body.SharePolicy,
		BackupPolicy:       avs.Body.BackupPolicy,
		ReclaimPolicy:      avs.Body.ReclaimPolicy,
		AllowExpansion:     avs.Body.AllowExpansion,
	}

	// TODO VolumeCapacity  AccessMode SharePolicy BackupPolicy ReclaimPolicy AllowExpansion 参数的校验

	if !strings.HasPrefix(avs.Body.VolumePath, "/") {
		httputil.ReturnError(r, w, 400, "volume path is invalid,must begin with /")
		return
	}
	if err := handler.GetServiceManager().VolumnVar(tsv, tenantID, avs.Body.FileContent, "add"); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//DeleteVolume DeleteVolume
func DeleteVolume(w http.ResponseWriter, r *http.Request) {
	// swagger:operation DELETE /v2/tenants/{tenant_name}/services/{service_alias}/volumes/{volume_name} v2 deleteVolumes
	//
	// 删除应用持久化信息(V2.1支持多种类型存储)
	//
	// delete volume
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式

	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	tenantID := r.Context().Value(middleware.ContextKey("tenant_id")).(string)
	tsv := &dbmodel.TenantServiceVolume{}
	tsv.ServiceID = serviceID
	tsv.VolumeName = chi.URLParam(r, "volume_name")
	if err := handler.GetServiceManager().VolumnVar(tsv, tenantID, "", "delete"); err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, nil)
}

//GetVolume 获取应用全部存储，包括依赖的存储
func GetVolume(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/tenants/{tenant_name}/services/{service_alias}/volumes v2 getVolumes
	//
	// 获取应用全部存储，包括依赖的存储(V2.1支持多种类型存储)
	//
	// get volumes
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	volumes, err := handler.GetServiceManager().GetVolumes(serviceID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, volumes)
}

//GetDepVolume 获取应用所有依赖的存储
func GetDepVolume(w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /v2/tenants/{tenant_name}/services/{service_alias}/depvolumes v2 getDepVolumes
	//
	// 获取应用依赖的存储(V2.1支持多种类型存储)
	//
	// get depvolumes
	//
	// ---
	// consumes:
	// - application/json
	// - application/x-protobuf
	//
	// produces:
	// - application/json
	// - application/xml
	//
	// responses:
	//   default:
	//     schema:
	//       "$ref": "#/responses/commandResponse"
	//     description: 统一返回格式
	serviceID := r.Context().Value(middleware.ContextKey("service_id")).(string)
	volumes, err := handler.GetServiceManager().GetDepVolumes(serviceID)
	if err != nil {
		err.Handle(r, w)
		return
	}
	httputil.ReturnSuccess(r, w, volumes)
}
