// Copyright 2015 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"time"

	"github.com/tsuru/tsuru/app"
	"github.com/tsuru/tsuru/permission"
	"github.com/tsuru/tsuru/provision"
	"github.com/tsuru/tsuru/tsurutest"
	"golang.org/x/net/websocket"
	"gopkg.in/check.v1"
)

func (s *S) TestAppShellWithAppName(c *check.C) {
	a := app.App{
		Name:      "someapp",
		Platform:  "zend",
		TeamOwner: s.team.Name,
	}
	err := app.CreateApp(&a, s.user)
	c.Assert(err, check.IsNil)
	err = s.provisioner.AddUnits(&a, 1, "web", nil)
	c.Assert(err, check.IsNil)
	m := RunServer(true)
	server := httptest.NewServer(m)
	defer server.Close()
	testServerURL, err := url.Parse(server.URL)
	c.Assert(err, check.IsNil)
	url := fmt.Sprintf("ws://%s/apps/%s/shell?width=140&height=38&term=xterm", testServerURL.Host, a.Name)
	config, err := websocket.NewConfig(url, "ws://localhost/")
	c.Assert(err, check.IsNil)
	config.Header.Set("Authorization", "bearer "+s.token.GetValue())
	wsConn, err := websocket.DialConfig(config)
	c.Assert(err, check.IsNil)
	defer wsConn.Close()
	_, err = wsConn.Write([]byte("echo test"))
	c.Assert(err, check.IsNil)
	var shells []provision.ShellOptions
	err = tsurutest.WaitCondition(5*time.Second, func() bool {
		units, unitsErr := s.provisioner.Units(&a)
		c.Assert(unitsErr, check.IsNil)
		unit := units[0]
		shells = s.provisioner.Shells(unit.ID)
		return len(shells) == 1
	})
	c.Assert(err, check.IsNil)
	c.Assert(shells[0].App.GetName(), check.Equals, a.Name)
	c.Assert(shells[0].Width, check.Equals, 140)
	c.Assert(shells[0].Height, check.Equals, 38)
	c.Assert(shells[0].Term, check.Equals, "xterm")
	c.Assert(shells[0].Unit, check.Equals, "")
}

func (s *S) TestAppShellWithAppNameInvalidPermission(c *check.C) {
	a := app.App{
		Name:      "someapp",
		Platform:  "zend",
		TeamOwner: s.team.Name,
	}
	err := app.CreateApp(&a, s.user)
	c.Assert(err, check.IsNil)
	err = s.provisioner.AddUnits(&a, 1, "web", nil)
	c.Assert(err, check.IsNil)
	m := RunServer(true)
	server := httptest.NewServer(m)
	defer server.Close()
	testServerURL, err := url.Parse(server.URL)
	c.Assert(err, check.IsNil)
	token := userWithPermission(c, permission.Permission{
		Scheme:  permission.PermAppRead,
		Context: permission.Context(permission.CtxApp, a.Name),
	})
	url := fmt.Sprintf("ws://%s/apps/%s/shell?width=140&height=38&term=xterm", testServerURL.Host, a.Name)
	config, err := websocket.NewConfig(url, "ws://localhost/")
	c.Assert(err, check.IsNil)
	config.Header.Set("Authorization", "bearer "+token.GetValue())
	wsConn, err := websocket.DialConfig(config)
	c.Assert(err, check.IsNil)
	defer wsConn.Close()
	_, err = wsConn.Write([]byte("echo test"))
	c.Assert(err, check.IsNil)
	var result string
	err = tsurutest.WaitCondition(5*time.Second, func() bool {
		part, readErr := ioutil.ReadAll(wsConn)
		if readErr != nil {
			return false
		}
		result += string(part)
		return result == "Error: You don't have permission to do this action\n"
	})
	c.Assert(err, check.IsNil)
}

func (s *S) TestAppShellSpecifyUnit(c *check.C) {
	a := app.App{
		Name:      "someapp",
		Platform:  "zend",
		TeamOwner: s.team.Name,
	}
	err := app.CreateApp(&a, s.user)
	c.Assert(err, check.IsNil)
	err = s.provisioner.AddUnits(&a, 5, "web", nil)
	c.Assert(err, check.IsNil)
	units, err := s.provisioner.Units(&a)
	c.Assert(err, check.IsNil)
	unit := units[3]
	m := RunServer(true)
	server := httptest.NewServer(m)
	defer server.Close()
	testServerURL, err := url.Parse(server.URL)
	c.Assert(err, check.IsNil)
	url := fmt.Sprintf("ws://%s/apps/%s/shell?width=140&height=38&term=xterm&unit=%s", testServerURL.Host, a.Name, unit.ID)
	config, err := websocket.NewConfig(url, "ws://localhost/")
	c.Assert(err, check.IsNil)
	config.Header.Set("Authorization", "bearer "+s.token.GetValue())
	wsConn, err := websocket.DialConfig(config)
	c.Assert(err, check.IsNil)
	defer wsConn.Close()
	_, err = wsConn.Write([]byte("echo test"))
	c.Assert(err, check.IsNil)
	var shells []provision.ShellOptions
	err = tsurutest.WaitCondition(5*time.Second, func() bool {
		shells = s.provisioner.Shells(unit.ID)
		return len(shells) == 1
	})
	c.Assert(err, check.IsNil)
	c.Assert(shells, check.HasLen, 1)
	c.Assert(shells[0].App.GetName(), check.Equals, a.Name)
	c.Assert(shells[0].Width, check.Equals, 140)
	c.Assert(shells[0].Height, check.Equals, 38)
	c.Assert(shells[0].Term, check.Equals, "xterm")
	c.Assert(shells[0].Unit, check.Equals, unit.ID)
	units, err = s.provisioner.Units(&a)
	c.Assert(err, check.IsNil)
	for _, u := range units {
		if u.ID != unit.ID {
			c.Check(s.provisioner.Shells(u.ID), check.HasLen, 0)
		}
	}
}

func (s *S) TestAppShellUnauthorizedError(c *check.C) {
	a := app.App{
		Name:      "someapp",
		Platform:  "zend",
		TeamOwner: s.team.Name,
	}
	err := app.CreateApp(&a, s.user)
	c.Assert(err, check.IsNil)
	err = s.provisioner.AddUnits(&a, 1, "web", nil)
	c.Assert(err, check.IsNil)
	m := RunServer(true)
	server := httptest.NewServer(m)
	defer server.Close()
	testServerURL, err := url.Parse(server.URL)
	c.Assert(err, check.IsNil)
	url := fmt.Sprintf("ws://%s/apps/%s/shell?width=140&height=38&term=xterm", testServerURL.Host, a.Name)
	config, err := websocket.NewConfig(url, "ws://localhost/")
	c.Assert(err, check.IsNil)
	wsConn, err := websocket.DialConfig(config)
	c.Assert(err, check.IsNil)
	defer wsConn.Close()
	_, err = wsConn.Write([]byte("echo test"))
	c.Assert(err, check.IsNil)
	var result string
	err = tsurutest.WaitCondition(5*time.Second, func() bool {
		part, readErr := ioutil.ReadAll(wsConn)
		if readErr != nil {
			return false
		}
		result += string(part)
		return result == "Error: no token provided or session expired, please login again\n"
	})
	c.Assert(err, check.IsNil)
}

func (s *S) TestAppShellGenericError(c *check.C) {
	m := RunServer(true)
	server := httptest.NewServer(m)
	defer server.Close()
	testServerURL, err := url.Parse(server.URL)
	c.Assert(err, check.IsNil)
	url := fmt.Sprintf("ws://%s/apps/someapp/shell?width=140&height=38&term=xterm", testServerURL.Host)
	config, err := websocket.NewConfig(url, "ws://localhost/")
	c.Assert(err, check.IsNil)
	config.Header.Set("Authorization", "bearer "+s.token.GetValue())
	wsConn, err := websocket.DialConfig(config)
	c.Assert(err, check.IsNil)
	defer wsConn.Close()
	_, err = wsConn.Write([]byte("echo test"))
	c.Assert(err, check.IsNil)
	var result string
	err = tsurutest.WaitCondition(5*time.Second, func() bool {
		part, readErr := ioutil.ReadAll(wsConn)
		if readErr != nil {
			c.Log(readErr)
			return false
		}
		result += string(part)
		return result == "Error: App someapp not found.\n"
	})
	c.Assert(err, check.IsNil)
}
