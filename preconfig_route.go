package main

import (
	"fmt"
	"regexp"
	"strings"
	"strconv"
)
type PreRouteItem struct {
	protocol string
	dest string
	host string
	port int
}

type PreConfigRoute struct {
	items map[string]*PreRouteItem
}

func NewPreRouteItem( protocol string, dest string, nextHop string ) (*PreRouteItem, error ) {
	pos := strings.LastIndex( nextHop, ":" )
	host := ""
	port := 5060
	if pos == -1 {
		host = nextHop
		if strings.EqualFold( "tls", protocol ) {
			port = 5061
		}
	} else {
		host = nextHop[0:pos]
		var err error
		port, err = strconv.Atoi( nextHop[ pos + 1:] )
		if err != nil {
			return nil, err
		}
	}
	return &PreRouteItem{ protocol: protocol,
	                      dest: dest,
	                      host: host,
		              port: port }, nil
}


func NewPreConfigRoute() *PreConfigRoute {
	return &PreConfigRoute{ items: make( map[string]*PreRouteItem ) }
}


func (pcr *PreConfigRoute) AddRouteItem( protocol string, dest string, nextHop string ) error {
	item, err := NewPreRouteItem( protocol, dest, nextHop )
	if err == nil {
		pcr.items[ dest ] = item
	}
	return err
}

func (pcr *PreConfigRoute) FindRoute( dest string )( protocol string, host string, port int, err error ) {
	if item, ok := pcr.items[ dest ]; ok {
		return item.protocol, item.host, item.port, nil
	}
	for _, item := range pcr.items {
		matched, err := regexp.MatchString( pcr.toRegularExp( item.dest ), dest )
		if matched && err == nil {
			return item.protocol, item.host, item.port, nil
		}
	}
	if item, ok := pcr.items[ "default" ]; ok {
		return item.protocol, item.host, item.port, nil
	}

	return "", "", 0, fmt.Errorf( "Fail to find route for %s", dest )
}

func (pcr *PreConfigRoute)toRegularExp( s string) string {
	s = strings.Replace( s, ".", "\\.", -1 )
	return fmt.Sprintf( "^%s$", strings.Replace( s, "*", ".*", -1 ) )
}
