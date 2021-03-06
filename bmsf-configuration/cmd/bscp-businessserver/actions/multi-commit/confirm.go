/*
Tencent is pleased to support the open source community by making Blueking Container Service available.
Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
Licensed under the MIT License (the "License"); you may not use this file except
in compliance with the License. You may obtain a copy of the License at
http://opensource.org/licenses/MIT
Unless required by applicable law or agreed to in writing, software distributed under
the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
either express or implied. See the License for the specific language governing permissions and
limitations under the License.
*/

package multicommit

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/viper"

	"bk-bscp/cmd/bscp-businessserver/modules/audit"
	"bk-bscp/internal/database"
	pb "bk-bscp/internal/protocol/businessserver"
	pbcommon "bk-bscp/internal/protocol/common"
	pbdatamanager "bk-bscp/internal/protocol/datamanager"
	pbtemplateserver "bk-bscp/internal/protocol/templateserver"
	"bk-bscp/pkg/common"
	"bk-bscp/pkg/logger"
)

// ConfirmAction confirms target multi commit object.
type ConfirmAction struct {
	viper          *viper.Viper
	dataMgrCli     pbdatamanager.DataManagerClient
	templateSvrCli pbtemplateserver.TemplateClient

	req  *pb.ConfirmMultiCommitReq
	resp *pb.ConfirmMultiCommitResp

	multiCommit *pbcommon.MultiCommit
	commitids   []string
}

// NewConfirmAction creates new ConfirmAction.
func NewConfirmAction(viper *viper.Viper, dataMgrCli pbdatamanager.DataManagerClient, templateSvrCli pbtemplateserver.TemplateClient,
	req *pb.ConfirmMultiCommitReq, resp *pb.ConfirmMultiCommitResp) *ConfirmAction {
	action := &ConfirmAction{viper: viper, dataMgrCli: dataMgrCli, templateSvrCli: templateSvrCli, req: req, resp: resp}

	action.resp.Seq = req.Seq
	action.resp.ErrCode = pbcommon.ErrCode_E_OK
	action.resp.ErrMsg = "OK"

	return action
}

// Err setup error code message in response and return the error.
func (act *ConfirmAction) Err(errCode pbcommon.ErrCode, errMsg string) error {
	act.resp.ErrCode = errCode
	act.resp.ErrMsg = errMsg
	return errors.New(errMsg)
}

// Input handles the input messages.
func (act *ConfirmAction) Input() error {
	if err := act.verify(); err != nil {
		return act.Err(pbcommon.ErrCode_E_BS_PARAMS_INVALID, err.Error())
	}
	return nil
}

// Output handles the output messages.
func (act *ConfirmAction) Output() error {
	// do nothing.
	return nil
}

func (act *ConfirmAction) verify() error {
	length := len(act.req.Bid)
	if length == 0 {
		return errors.New("invalid params, bid missing")
	}
	if length > database.BSCPIDLENLIMIT {
		return errors.New("invalid params, bid too long")
	}

	length = len(act.req.MultiCommitid)
	if length == 0 {
		return errors.New("invalid params, multi commitid missing")
	}
	if length > database.BSCPIDLENLIMIT {
		return errors.New("invalid params, multi commitid too long")
	}

	length = len(act.req.Operator)
	if length == 0 {
		return errors.New("invalid params, operator missing")
	}
	if length > database.BSCPNAMELENLIMIT {
		return errors.New("invalid params, operator too long")
	}
	return nil
}

func (act *ConfirmAction) queryCommit(commitid string) (*pbcommon.Commit, pbcommon.ErrCode, string) {
	r := &pbdatamanager.QueryCommitReq{
		Seq:      act.req.Seq,
		Bid:      act.req.Bid,
		Commitid: commitid,
	}

	ctx, cancel := context.WithTimeout(context.Background(), act.viper.GetDuration("datamanager.calltimeout"))
	defer cancel()

	logger.V(2).Infof("ConfirmMultiCommit[%d]| request to datamanager QueryCommit, %+v", act.req.Seq, r)

	resp, err := act.dataMgrCli.QueryCommit(ctx, r)
	if err != nil {
		return nil, pbcommon.ErrCode_E_BS_SYSTEM_UNKONW, fmt.Sprintf("request to datamanager QueryCommit, %+v", err)
	}
	return resp.Commit, resp.ErrCode, resp.ErrMsg
}

func (act *ConfirmAction) render(commit *pbcommon.Commit) (pbcommon.ErrCode, string) {
	r := &pbtemplateserver.RenderReq{
		Seq:      act.req.Seq,
		Bid:      act.req.Bid,
		Appid:    commit.Appid,
		Commitid: commit.Commitid,
		Operator: act.req.Operator,
	}

	ctx, cancel := context.WithTimeout(context.Background(), act.viper.GetDuration("templateserver.calltimeout"))
	defer cancel()

	logger.V(2).Infof("ConfirmMultiCommit[%d]| request to templateserver Render, %+v", act.req.Seq, r)

	resp, err := act.templateSvrCli.Render(ctx, r)
	if err != nil {
		return pbcommon.ErrCode_E_BS_SYSTEM_UNKONW, fmt.Sprintf("request to templateserver Render, %+v", err)
	}
	return resp.ErrCode, resp.ErrMsg
}

func (act *ConfirmAction) renderWithoutTemplate(commit *pbcommon.Commit) (pbcommon.ErrCode, string) {
	r := &pbdatamanager.CreateConfigsReq{
		Seq:      act.req.Seq,
		Bid:      act.req.Bid,
		Appid:    commit.Appid,
		Cfgsetid: commit.Cfgsetid,
		Commitid: commit.Commitid,
		Cid:      common.SHA256(string(commit.Configs)),
		CfgLink:  "",
		Content:  commit.Configs,
		Creator:  act.req.Operator,
	}

	ctx, cancel := context.WithTimeout(context.Background(), act.viper.GetDuration("datamanager.calltimeout"))
	defer cancel()

	logger.V(2).Infof("ConfirmMultiCommit[%d]| request to datamanager CreateConfigs, %+v", act.req.Seq, r)

	resp, err := act.dataMgrCli.CreateConfigs(ctx, r)
	if err != nil {
		return pbcommon.ErrCode_E_BS_SYSTEM_UNKONW, fmt.Sprintf("request to datamanager CreateConfigs, %+v", err)
	}
	return resp.ErrCode, resp.ErrMsg
}

func (act *ConfirmAction) confirmCommit(commit *pbcommon.Commit) (pbcommon.ErrCode, string) {
	r := &pbdatamanager.ConfirmCommitReq{
		Seq:      act.req.Seq,
		Bid:      act.req.Bid,
		Appid:    commit.Appid,
		Cfgsetid: commit.Cfgsetid,
		Commitid: commit.Commitid,
		Operator: act.req.Operator,
	}

	ctx, cancel := context.WithTimeout(context.Background(), act.viper.GetDuration("datamanager.calltimeout"))
	defer cancel()

	logger.V(2).Infof("ConfirmMultiCommit[%d]| request to datamanager ConfirmCommit, %+v", act.req.Seq, r)

	resp, err := act.dataMgrCli.ConfirmCommit(ctx, r)
	if err != nil {
		return pbcommon.ErrCode_E_BS_SYSTEM_UNKONW, fmt.Sprintf("request to datamanager ConfirmCommit, %+v", err)
	}
	if resp.ErrCode != pbcommon.ErrCode_E_OK {
		return resp.ErrCode, resp.ErrMsg
	}

	// audit here on commit confirmed.
	audit.Audit(int32(pbcommon.SourceType_ST_COMMIT), int32(pbcommon.SourceOpType_SOT_CONFIRM),
		act.req.Bid, commit.Commitid, act.req.Operator, "")

	return resp.ErrCode, resp.ErrMsg
}

func (act *ConfirmAction) confirmMultiCommit() (pbcommon.ErrCode, string) {
	r := &pbdatamanager.ConfirmMultiCommitReq{
		Seq:           act.req.Seq,
		Bid:           act.req.Bid,
		MultiCommitid: act.req.MultiCommitid,
		Operator:      act.req.Operator,
	}

	ctx, cancel := context.WithTimeout(context.Background(), act.viper.GetDuration("datamanager.calltimeout"))
	defer cancel()

	logger.V(2).Infof("ConfirmMultiCommit[%d]| request to datamanager ConfirmMultiCommit, %+v", act.req.Seq, r)

	resp, err := act.dataMgrCli.ConfirmMultiCommit(ctx, r)
	if err != nil {
		return pbcommon.ErrCode_E_BS_SYSTEM_UNKONW, fmt.Sprintf("request to datamanager ConfirmMultiCommit, %+v", err)
	}
	if resp.ErrCode != pbcommon.ErrCode_E_OK {
		return resp.ErrCode, resp.ErrMsg
	}

	// audit here on commit confirmed.
	audit.Audit(int32(pbcommon.SourceType_ST_MULTI_COMMIT), int32(pbcommon.SourceOpType_SOT_CONFIRM),
		act.req.Bid, act.req.MultiCommitid, act.req.Operator, "")

	return resp.ErrCode, resp.ErrMsg
}

func (act *ConfirmAction) querySubCommitList() (pbcommon.ErrCode, string) {
	r := &pbdatamanager.QueryMultiCommitSubListReq{
		Seq:           act.req.Seq,
		Bid:           act.req.Bid,
		MultiCommitid: act.req.MultiCommitid,
	}

	ctx, cancel := context.WithTimeout(context.Background(), act.viper.GetDuration("datamanager.calltimeout"))
	defer cancel()

	logger.V(2).Infof("ConfirmMultiCommit[%d]| request to datamanager QueryMultiCommitSubList, %+v", act.req.Seq, r)

	resp, err := act.dataMgrCli.QueryMultiCommitSubList(ctx, r)
	if err != nil {
		return pbcommon.ErrCode_E_BS_SYSTEM_UNKONW, fmt.Sprintf("request to datamanager QueryMultiCommitSubList, %+v", err)
	}
	act.commitids = resp.Commitids

	if resp.ErrCode != pbcommon.ErrCode_E_OK {
		return resp.ErrCode, resp.ErrMsg
	}

	return resp.ErrCode, resp.ErrMsg
}

// Do makes the workflows of this action base on input messages.
func (act *ConfirmAction) Do() error {
	// query multi commit sub commit list.
	if errCode, errMsg := act.querySubCommitList(); errCode != pbcommon.ErrCode_E_OK {
		return act.Err(errCode, errMsg)
	}

	for _, commitid := range act.commitids {
		// query target sub commit.
		commit, errCode, errMsg := act.queryCommit(commitid)
		if errCode != pbcommon.ErrCode_E_OK {
			return act.Err(errCode, errMsg)
		}

		// already confirmed.
		if commit.State == int32(pbcommon.CommitState_CS_CONFIRMED) {
			continue
		}

		// already canceled.
		if commit.State == int32(pbcommon.CommitState_CS_CANCELED) {
			return act.Err(pbcommon.ErrCode_E_BS_COMMIT_ALREADY_CANCELED,
				fmt.Sprintf("can't confirm the sub commit[%s] which is already canceled.", commitid))
		}

		// rendering configs.
		if len(commit.Template) != 0 {
			if errCode, errMsg := act.render(commit); errCode != pbcommon.ErrCode_E_OK {
				return act.Err(errCode, errMsg)
			}
		} else {
			if errCode, errMsg := act.renderWithoutTemplate(commit); errCode != pbcommon.ErrCode_E_OK {
				return act.Err(errCode, errMsg)
			}
		}

		// confirm commit.
		if errCode, errMsg := act.confirmCommit(commit); errCode != pbcommon.ErrCode_E_OK {
			return act.Err(errCode, errMsg)
		}
	}

	// confirm multi commit.
	if errCode, errMsg := act.confirmMultiCommit(); errCode != pbcommon.ErrCode_E_OK {
		return act.Err(errCode, errMsg)
	}
	return nil
}
