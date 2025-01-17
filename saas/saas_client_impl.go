package saas

import (
	"errors"
	"fmt"
	"github.com/byteplus-sdk/sdk-go/common"
	. "github.com/byteplus-sdk/sdk-go/core"
	"github.com/byteplus-sdk/sdk-go/core/logs"
	"github.com/byteplus-sdk/sdk-go/core/option"
	"github.com/byteplus-sdk/sdk-go/saas/protocol"
	"strings"
)

var (
	writeMsgFormat  = "Only can receive max to %d items in one write request"
	writeTooManyErr = errors.New(fmt.Sprintf(writeMsgFormat, MaxImportWriteCount))
)

const (
	dataInitOptionCount    = 1
	predictInitOptionCount = 1
)

type clientImpl struct {
	common.Client
	hCaller *HttpCaller
	su      *saasURL
	hostAva *HostAvailabler
}

func (c *clientImpl) Release() {
	c.hostAva.Shutdown()
}

func checkProjectIdAndModelId(projectId string, modelId string) error {
	const (
		errMsgFormat      = "%s,field can not empty"
		errFieldProjectId = "projectId"
		errFieldModelId   = "modelId"
	)
	if projectId != "" && modelId != "" {
		return nil
	}
	emptyParams := make([]string, 0)
	if projectId == "" {
		emptyParams = append(emptyParams, errFieldProjectId)
	}
	if modelId == "" {
		emptyParams = append(emptyParams, errFieldModelId)
	}
	return errors.New(fmt.Sprintf(errMsgFormat, strings.Join(emptyParams, ",")))
}

func checkProjectIdAndStage(projectId string, stage string) error {
	const (
		errMsgFormat      = "%s,field can not empty"
		errFieldProjectId = "projectId"
		errFieldStage     = "stage"
	)
	if projectId != "" && stage != "" {
		return nil
	}
	emptyParams := make([]string, 0)
	if projectId == "" {
		emptyParams = append(emptyParams, errFieldProjectId)
	}
	if stage == "" {
		emptyParams = append(emptyParams, errFieldStage)
	}
	return errors.New(fmt.Sprintf(errMsgFormat, strings.Join(emptyParams, ",")))
}

func addSaasFlag(opts []option.Option) []option.Option {
	return append(opts, withSaasHeader())
}

func withSaasHeader() option.Option {
	const (
		HTTPHeaderServerFrom = "Server-From"
		SaasFlag             = "saas"
	)
	return func(opt *option.Options) {
		if len(opt.Headers) == 0 {
			opt.Headers = map[string]string{HTTPHeaderServerFrom: SaasFlag}
			return
		}
		opt.Headers[HTTPHeaderServerFrom] = SaasFlag
	}
}

func (c *clientImpl) doWrite(request *protocol.WriteDataRequest, url string, opts ...option.Option) (*protocol.WriteResponse, error) {
	if err := checkProjectIdAndStage(request.ProjectId, request.Stage); err != nil {
		return nil, err
	}
	if len(request.GetData()) > MaxImportWriteCount {
		return nil, writeTooManyErr
	}
	if len(opts) == 0 {
		opts = make([]option.Option, 0, dataInitOptionCount)
	}
	response := &protocol.WriteResponse{}
	opts = addSaasFlag(opts)
	err := c.hCaller.DoPbRequest(url, request, response, option.Conv2Options(opts...))
	if err != nil {
		return nil, err
	}
	logs.Debug("[WriteData] rsp:\n%s\n", response)
	return response, nil
}

func (c *clientImpl) WriteUsers(writeRequest *protocol.WriteDataRequest, opts ...option.Option) (*protocol.WriteResponse, error) {
	return c.doWrite(writeRequest, c.su.writeUsersURL, opts...)
}

func (c *clientImpl) WriteProducts(writeRequest *protocol.WriteDataRequest, opts ...option.Option) (*protocol.WriteResponse, error) {
	return c.doWrite(writeRequest, c.su.writeProductsURL, opts...)
}

func (c *clientImpl) WriteUserEvents(writeRequest *protocol.WriteDataRequest, opts ...option.Option) (*protocol.WriteResponse, error) {
	return c.doWrite(writeRequest, c.su.writeUserEventsURL, opts...)
}

func (c *clientImpl) Predict(request *protocol.PredictRequest, opts ...option.Option) (*protocol.PredictResponse, error) {
	if err := checkProjectIdAndModelId(request.ProjectId, request.ModelId); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		opts = make([]option.Option, 0, predictInitOptionCount)
	}
	response := &protocol.PredictResponse{}
	opts = addSaasFlag(opts)
	err := c.hCaller.DoPbRequest(c.su.predictURL, request, response, option.Conv2Options(opts...))
	if err != nil {
		return nil, err
	}
	logs.Debug("[Predict] rsp:\n%s\n", response)
	return response, nil
}

func (c *clientImpl) AckServerImpressions(request *protocol.AckServerImpressionsRequest,
	opts ...option.Option) (*protocol.AckServerImpressionsResponse, error) {
	if err := checkProjectIdAndModelId(request.ProjectId, request.ModelId); err != nil {
		return nil, err
	}
	if len(opts) == 0 {
		opts = make([]option.Option, 0, predictInitOptionCount)
	}
	response := &protocol.AckServerImpressionsResponse{}
	opts = addSaasFlag(opts)
	err := c.hCaller.DoPbRequest(c.su.ackImpressionURL, request, response, option.Conv2Options(opts...))
	if err != nil {
		return nil, err
	}
	logs.Debug("[AckImpressions] rsp:\n%s\n", response)
	return response, nil
}
