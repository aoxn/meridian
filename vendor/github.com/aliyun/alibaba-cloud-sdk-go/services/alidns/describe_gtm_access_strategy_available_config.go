package alidns

//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
// Code generated by Alibaba Cloud SDK Code Generator.
// Changes may cause incorrect behavior and will be lost if the code is regenerated.

import (
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/responses"
)

// DescribeGtmAccessStrategyAvailableConfig invokes the alidns.DescribeGtmAccessStrategyAvailableConfig API synchronously
func (client *Client) DescribeGtmAccessStrategyAvailableConfig(request *DescribeGtmAccessStrategyAvailableConfigRequest) (response *DescribeGtmAccessStrategyAvailableConfigResponse, err error) {
	response = CreateDescribeGtmAccessStrategyAvailableConfigResponse()
	err = client.DoAction(request, response)
	return
}

// DescribeGtmAccessStrategyAvailableConfigWithChan invokes the alidns.DescribeGtmAccessStrategyAvailableConfig API asynchronously
func (client *Client) DescribeGtmAccessStrategyAvailableConfigWithChan(request *DescribeGtmAccessStrategyAvailableConfigRequest) (<-chan *DescribeGtmAccessStrategyAvailableConfigResponse, <-chan error) {
	responseChan := make(chan *DescribeGtmAccessStrategyAvailableConfigResponse, 1)
	errChan := make(chan error, 1)
	err := client.AddAsyncTask(func() {
		defer close(responseChan)
		defer close(errChan)
		response, err := client.DescribeGtmAccessStrategyAvailableConfig(request)
		if err != nil {
			errChan <- err
		} else {
			responseChan <- response
		}
	})
	if err != nil {
		errChan <- err
		close(responseChan)
		close(errChan)
	}
	return responseChan, errChan
}

// DescribeGtmAccessStrategyAvailableConfigWithCallback invokes the alidns.DescribeGtmAccessStrategyAvailableConfig API asynchronously
func (client *Client) DescribeGtmAccessStrategyAvailableConfigWithCallback(request *DescribeGtmAccessStrategyAvailableConfigRequest, callback func(response *DescribeGtmAccessStrategyAvailableConfigResponse, err error)) <-chan int {
	result := make(chan int, 1)
	err := client.AddAsyncTask(func() {
		var response *DescribeGtmAccessStrategyAvailableConfigResponse
		var err error
		defer close(result)
		response, err = client.DescribeGtmAccessStrategyAvailableConfig(request)
		callback(response, err)
		result <- 1
	})
	if err != nil {
		defer close(result)
		callback(nil, err)
		result <- 0
	}
	return result
}

// DescribeGtmAccessStrategyAvailableConfigRequest is the request struct for api DescribeGtmAccessStrategyAvailableConfig
type DescribeGtmAccessStrategyAvailableConfigRequest struct {
	*requests.RpcRequest
	InstanceId   string `position:"Query" name:"InstanceId"`
	UserClientIp string `position:"Query" name:"UserClientIp"`
	Lang         string `position:"Query" name:"Lang"`
}

// DescribeGtmAccessStrategyAvailableConfigResponse is the response struct for api DescribeGtmAccessStrategyAvailableConfig
type DescribeGtmAccessStrategyAvailableConfigResponse struct {
	*responses.BaseResponse
	RequestId             string                                              `json:"RequestId" xml:"RequestId"`
	SuggestSetDefaultLine bool                                                `json:"SuggestSetDefaultLine" xml:"SuggestSetDefaultLine"`
	AddrPools             AddrPoolsInDescribeGtmAccessStrategyAvailableConfig `json:"AddrPools" xml:"AddrPools"`
	Lines                 LinesInDescribeGtmAccessStrategyAvailableConfig     `json:"Lines" xml:"Lines"`
}

// CreateDescribeGtmAccessStrategyAvailableConfigRequest creates a request to invoke DescribeGtmAccessStrategyAvailableConfig API
func CreateDescribeGtmAccessStrategyAvailableConfigRequest() (request *DescribeGtmAccessStrategyAvailableConfigRequest) {
	request = &DescribeGtmAccessStrategyAvailableConfigRequest{
		RpcRequest: &requests.RpcRequest{},
	}
	request.InitWithApiInfo("Alidns", "2015-01-09", "DescribeGtmAccessStrategyAvailableConfig", "alidns", "openAPI")
	request.Method = requests.POST
	return
}

// CreateDescribeGtmAccessStrategyAvailableConfigResponse creates a response to parse from DescribeGtmAccessStrategyAvailableConfig response
func CreateDescribeGtmAccessStrategyAvailableConfigResponse() (response *DescribeGtmAccessStrategyAvailableConfigResponse) {
	response = &DescribeGtmAccessStrategyAvailableConfigResponse{
		BaseResponse: &responses.BaseResponse{},
	}
	return
}
