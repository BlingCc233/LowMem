package napcat

import (
	"encoding/json"
	"fmt"
)

// GetLoginInfo gets current login info
func (c *Client) GetLoginInfo() (*LoginInfo, error) {
	resp, err := c.Call("get_login_info", nil)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("api error: %s", resp.Message)
	}

	var info LoginInfo
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		return nil, fmt.Errorf("unmarshal login info: %w", err)
	}
	return &info, nil
}

// GetFriendList gets friend list
func (c *Client) GetFriendList() ([]Friend, error) {
	resp, err := c.Call("get_friend_list", nil)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("api error: %s", resp.Message)
	}

	var friends []Friend
	if err := json.Unmarshal(resp.Data, &friends); err != nil {
		return nil, fmt.Errorf("unmarshal friends: %w", err)
	}
	return friends, nil
}

// GetGroupList gets group list
func (c *Client) GetGroupList() ([]Group, error) {
	resp, err := c.Call("get_group_list", nil)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("api error: %s", resp.Message)
	}

	var groups []Group
	if err := json.Unmarshal(resp.Data, &groups); err != nil {
		return nil, fmt.Errorf("unmarshal groups: %w", err)
	}
	return groups, nil
}

// SendPrivateMsg sends a private message
func (c *Client) SendPrivateMsg(userID int64, message []MessageSegment) (*SendMsgResponse, error) {
	params := map[string]interface{}{
		"user_id": userID,
		"message": message,
	}

	resp, err := c.Call("send_private_msg", params)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("api error: %s", resp.Message)
	}

	var result SendMsgResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// SendGroupMsg sends a group message
func (c *Client) SendGroupMsg(groupID int64, message []MessageSegment) (*SendMsgResponse, error) {
	params := map[string]interface{}{
		"group_id": groupID,
		"message":  message,
	}

	resp, err := c.Call("send_group_msg", params)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("api error: %s", resp.Message)
	}

	var result SendMsgResponse
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// DeleteMsg recalls/deletes a message
func (c *Client) DeleteMsg(messageID int32) error {
	params := map[string]interface{}{
		"message_id": messageID,
	}

	resp, err := c.Call("delete_msg", params)
	if err != nil {
		return err
	}
	if !resp.IsOK() {
		return fmt.Errorf("api error: %s", resp.Message)
	}
	return nil
}

// GetMsg gets a message by ID
func (c *Client) GetMsg(messageID int32) (map[string]interface{}, error) {
	params := map[string]interface{}{
		"message_id": messageID,
	}

	resp, err := c.Call("get_msg", params)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("api error: %s", resp.Message)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return result, nil
}

// GetStrangerInfo gets stranger info by user ID
func (c *Client) GetStrangerInfo(userID int64) (*Sender, error) {
	params := map[string]interface{}{
		"user_id": userID,
	}

	resp, err := c.Call("get_stranger_info", params)
	if err != nil {
		return nil, err
	}
	if !resp.IsOK() {
		return nil, fmt.Errorf("api error: %s", resp.Message)
	}

	var info Sender
	if err := json.Unmarshal(resp.Data, &info); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &info, nil
}
