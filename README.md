#SkyDNS
*Version 0.1.0*

SkyDNS is a distributed service for announcement and discovery of services. It leverages Raft for high-availability and consensus, and utilizes DNS for queries to discover available services. This is done by leveraging SRV records in DNS, with special meaning given to subdomains, priorities and weights.

[Announcement Blog Post](http://blog.gopheracademy.com/skydns)

##Setup / Install

Compile SkyDNS, and execute it

`./build.sh`

`./skydns`

Which takes the following flags
- -domain - This is the domain requests are anchored to and should be appended to all requests (Defaults to: skydns.local)
- -http - This is the HTTP ip:port to listen on for API request (Defaults to: 127.0.0.1:8080)
- -dns - This is the ip:port to listen on for DNS requests (Defaults to: 127.0.0.1:53)
- -data - Directory that Raft logs will be stored in (Defaults to: ./data)
- -join - When running a cluster of SkyDNS servers as recommended, you'll need to supply followers with where the other members can be found, this can be any member or a comma separated list of members. It does not have to be the leader. Any non-leader you join will redirect you to the leader automatically.
- -discover - This flag can be used in place of explicitly supplying cluster members via the -join flag. It performs a DNS lookup using the hosts DNS server for NS records associated with the -domain flag to find the SkyDNS instances.

##API
### Service Announcements
You announce your service by submitting JSON over HTTP to SkyDNS with information about your service.

`curl -X PUT -L http://localhost:8080/skydns/services/1001 -d '{"Name":"TestService","Version":"1.0.0","Environment":"Production","Region":"Test","Host":"web1.site.com","Port":9000,"TTL":10}'`

If successful you should receive an http status code of: **201 Created**

If a service with this UUID already exists you will receive back an http status code of: **409 Conflict**

SkyDNS will now have an entry for your service that will live for the number of seconds supplied in your TTL (10 seconds in our example), unless you send a  heartbeat to update the TTL.

### Heartbeat / Keep alive
SkyDNS requires that services submit an HTTP request to update their TTL within the TTL they last supplied. If the service fails to do so within this timeframe SkyDNS will expire the service automatically. This will allow for nodes to fail and DNS to reflect this quickly.

You can update your TTL by sending an HTTP request to SkyDNS with an updated TTL, it can be the same as before to allow it to live for another 10s, or it can be adjusted to a shorter or longer duration.

`curl -X PATCH -L http://localhost:8080/skydns/services/1001 -d '{"TTL":10}'`

### Service Removal
If you wish to remove your service from SkyDNS for any reason without waiting for the TTL to expire, you simply send an HTTP DELETE.

`curl -X DELETE -L http://localhost:8080/skydns/services/1001`

### Retrieve Service Info via API
Currently you may only retrieve a service's info by UUID of the service, in the future we may implement querying of the services similar to the DNS  interface.

`curl -X GET -L http://localhost:8080/skydns/services/1001`

##Discovery (DNS)
You can find services by querying SkyDNS via any DNS client or utility. It uses a known domain syntax with wildcards to find matching services.

Priorities and Weights are based on the requested Region, as well as how many nodes are available matching the current request in the given region.

###Domain Format
The domain syntax when querying follows a pattern where the right most positions are more generic, than the subdomains to their left: *\<uuid\>.\<host\>.\<region\>.\<version\>.\<service\>.\<environment\>.skydns.local*. This allows for you to supply only the positions you care about:

- authservice.production.skydns.local - For instance would return all services with the name AuthService in the production environment, regardless of the Version, Region, or Host
- 1-0-0.authservice.production.skydns.local - Is the same as above but restricting it to only version 1.0.0
- east.1-0-0.authservice.production.skydns.local - Would add the restriction that the services must be running in the East region

In addition to only needing to specify as much of the domain as required for the granularity level you're looking for, you may also supply the wildcard **any** or **all** in any of the positions.

- east.any.any.production.skydns.local - Would return all services in the East region, that are a part of the production environment.

###Examples

Let's take a look at some results. First we need to add a few services so we have services to query against.

	// Service 1001 (East Region)
	curl -X PUT -L http://localhost:8080/skydns/services/1001 -d '{"Name":"TestService","Version":"1.0.0","Environment":"Production","Region":"East","Host":"web1.site.com","Port":80,"TTL":4000}'
	
	// Service 1002 (East Region)
	curl -X PUT -L http://localhost:8080/skydns/services/1002 -d '{"Name":"TestService","Version":"1.0.0","Environment":"Production","Region":"East","Host":"web2.site.com","Port":8080,"TTL":4000}'
	
	// Service 1003 (West Region)
	curl -X PUT -L http://localhost:8080/skydns/services/1003 -d '{"Name":"TestService","Version":"1.0.0","Environment":"Production","Region":"West","Host":"web3.site.com","Port":80,"TTL":4000}'
	
	// Service 1004 (West Region)
	curl -X PUT -L http://localhost:8080/skydns/services/1004 -d '{"Name":"TestService","Version":"1.0.0","Environment":"Production","Region":"West","Host":"web4.site.com","Port":80,"TTL":4000}'

Now we can try some of our example DNS lookups:
#####All services in the Production Environment
`dig @localhost production.skydns.local SRV`

	;; QUESTION SECTION:
	;production.skydns.local.			IN	SRV

	;; ANSWER SECTION:
	production.skydns.local.		629		IN	SRV	10 20 80   web1.site.com.
	production.skydns.local.		3979	IN	SRV	10 20 8080 web2.site.com.
	production.skydns.local.		3629	IN	SRV	10 20 9000 server24.
	production.skydns.local.		3985	IN	SRV	10 20 80   web3.site.com.
	production.skydns.local.		3990	IN	SRV	10 20 80   web4.site.com.

	;; Query time: 1 msec
	;; SERVER: 127.0.0.1#53(127.0.0.1)
	;; WHEN: Thu Oct 10 11:47:08 EDT 2013
	;; MSG SIZE  rcvd: 238
	
#####All TestService instances in Production Environment
`dig @localhost testservice.production.skydns.local SRV`

	;; QUESTION SECTION:
	;testservice.production.skydns.local.		IN	SRV

	;; ANSWER SECTION:
	testservice.production.skydns.local.	615		IN	SRV	10 20 80   web1.site.com.
	testservice.production.skydns.local.	3966	IN	SRV	10 20 8080 web2.site.com.
	testservice.production.skydns.local.	3615	IN	SRV	10 20 9000 server24.
	testservice.production.skydns.local.	3972	IN	SRV	10 20 80   web3.site.com.
	testservice.production.skydns.local.	3976	IN	SRV	10 20 80   web4.site.com.

	;; Query time: 0 msec
	;; SERVER: 127.0.0.1#53(127.0.0.1)
	;; WHEN: Thu Oct 10 11:47:22 EDT 2013
	;; MSG SIZE  rcvd: 310
	
#####All TestService v1.0.0 Instances in Production Environment
`dig @localhost 1-0-0.testservice.production.skydns.local SRV`

	;; QUESTION SECTION:
	;1-0-0.testservice.production.skydns.local.	IN	SRV

	;; ANSWER SECTION:
	1-0-0.testservice.production.skydns.local. 600  IN	SRV	10 20 80   web1.site.com.
	1-0-0.testservice.production.skydns.local. 3950 IN	SRV	10 20 8080 web2.site.com.
	1-0-0.testservice.production.skydns.local. 3600 IN	SRV	10 20 9000 server24.
	1-0-0.testservice.production.skydns.local. 3956 IN	SRV	10 20 80   web3.site.com.
	1-0-0.testservice.production.skydns.local. 3961 IN	SRV	10 20 80   web4.site.com.

	;; Query time: 0 msec
	;; SERVER: 127.0.0.1#53(127.0.0.1)
	;; WHEN: Thu Oct 10 11:47:37 EDT 2013
	;; MSG SIZE  rcvd: 346
	
#####All TestService Instances at any version, within the East region
`dig @localhost east.any.testservice.production.skydns.local SRV`

This is where we've changed things up a bit, notice we used the "any" wildcard for version so we get any version, and because we've supplied an explicit region that we're looking for we get that as the highest DNS priority, with the weight being distributed evenly, then all of our West instances still show up for fail-over, but with a higher Priority.

	;; QUESTION SECTION:
	;east.any.testservice.production.skydns.local. IN	SRV

	;; ANSWER SECTION:
	east.any.testservice.production.skydns.local. 531  IN SRV	10 50 80   web1.site.com.
	east.any.testservice.production.skydns.local. 3881 IN SRV	10 50 8080 web2.site.com.
	east.any.testservice.production.skydns.local. 3531 IN SRV	20 33 9000 server24.
	east.any.testservice.production.skydns.local. 3887 IN SRV	20 33 80   web3.site.com.
	east.any.testservice.production.skydns.local. 3892 IN SRV	20 33 80   web4.site.com.

	;; Query time: 0 msec
	;; SERVER: 127.0.0.1#53(127.0.0.1)
	;; WHEN: Thu Oct 10 11:48:46 EDT 2013
	;; MSG SIZE  rcvd: 364

##Roadmap
As awesome as SkyDNS is in its current state we plan to further development with the following.

* Handle Priorities for explicitly supplied hosts/uuids. This brings up an interesting question, if you supply a host or uuid should it only return exact matches, or should it work similar to regions where your supplied value gets a higher priority than others.
* More comprehensive test suite
* Validation of services
* Benchmarks / Performance Improvements
* Priorities based on latency between the requested region, and the additional external regions, as well as load in the given regions
* Weights based on system load/memory availability on the given host, so that idle nodes recieve a higher weight and therefore a larger percentage of the requests.
* Semantic version lookups (1.0 - returns any 1.0.x, 1.1.0 - Explicit 1.0.0, 1 - returns any 1.x.x), possibly priorities based on versions?
* Support for peers that don't participate in consensus

## License
The MIT License (MIT)

Copyright © 2013 Erik St. Martin, Brian Ketelsen

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
