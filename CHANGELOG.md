# CHANGELOG

## v0.1.3 (alpha) - December 13th 2017
* Support clusters where nodeName is the private or public IP (@klausenbusk)
* Switch Docker base image to Alpine from Ubuntu (@klausenbusk)

Supports Kubernetes Versions: v1.8

## v0.1.2 (alpha) - October 5th 2017
* Implement InstanceExistsByProviderID (@andrewsykim)
* Cloud Controller Manager should run as a critical pod with resource requests (@andrewsykim)
* Handle new provider ID format in node spec - digitalocean://droplet-id (@andrewsykim)
* Implement GetZoneByProviderID and GetZoneByNodeName (@bhcleek)
* Remove import for in-tree cloud provider - results in smaller binary (@andrewsykim)

Supports Kubernetes Versions: v1.8

## v0.1.1 (alpha) - September 27th 2017
* Wait for load balancer to be active to retrieve its IP (@odacremolbap)
* Use pagination when listing all droplets (@yuvalsade)

Supports Kubernetes Versions: v1.7

## v0.1.0 (alpha) - August 10th 2017
* implement nodecontroller - responsible for: address managemnet, monitoring node status and node deletions.
* implement zones - responsible for assigning nodes a zone in DigitalOcean
* implement servicecontroller - responsible for: creating, updating and deleting services of type `LoadBalancer` with DO loadbalancers.

Supports Kubernetes Versions: v1.7
