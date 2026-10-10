package sonar

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Channels are Sonar's classic-mode channel ids, in display order.
var Channels = [...]string{"Master", "game", "chatRender", "media", "aux", "chatCapture"}

type Level struct {
	Volume float64 `json:"volume"` // 0..1
	Muted  bool    `json:"muted"`
}

// State is everything the desktop mixer shows.
type State struct {
	Levels     [len(Channels)]Level
	ChatMix    float64 // -1 (game) .. 1 (chat)
	HasChatMix bool
}

func (c *Client) State() (State, error) {
	var vs struct {
		Masters struct {
			Classic Level `json:"classic"`
		} `json:"masters"`
		Devices map[string]struct {
			Classic Level `json:"classic"`
		} `json:"devices"`
	}
	if err := c.do(http.MethodGet, "/volumeSettings/classic", &vs); err != nil {
		return State{}, err
	}
	var s State
	s.Levels[0] = vs.Masters.Classic
	for i, id := range Channels[1:] {
		s.Levels[i+1] = vs.Devices[id].Classic
	}
	var mix struct {
		Balance float64 `json:"balance"`
	}
	if c.do(http.MethodGet, "/v1/chatMix", &mix) == nil {
		s.ChatMix, s.HasChatMix = mix.Balance, true
	}
	return s, nil
}

func (c *Client) SetVolume(channel string, v float64) error {
	return c.do(http.MethodPut, "/volumeSettings/classic/"+channel+"/Volume/"+strconv.FormatFloat(v, 'f', 4, 64), nil)
}

func (c *Client) SetMute(channel string, muted bool) error {
	return c.do(http.MethodPut, "/volumeSettings/classic/"+channel+"/Mute/"+strconv.FormatBool(muted), nil)
}

func (c *Client) SetChatMix(balance float64) error {
	return c.do(http.MethodPut, "/v1/chatMix?balance="+strconv.FormatFloat(balance, 'f', 4, 64), nil)
}

// do calls Sonar's API directly, discovering its address on first use. The
// client's transport re-discovers and retries once if Sonar has restarted.
func (c *Client) do(method, path string, out any) error {
	base := c.Base()
	if base == nil {
		if err := c.Discover(); err != nil {
			return err
		}
		base = c.Base()
	}
	req, err := http.NewRequest(method, base.Scheme+"://"+base.Host+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.api.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s %s: %s", method, path, resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
