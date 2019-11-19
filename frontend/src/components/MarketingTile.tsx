import * as React from 'react';
import { Card, CardHead, CardHeader, CardBody, CardFooter } from '@patternfly/react-core';

export interface TileProps {
    imgSrc: string;
    header: string;
    body: string;
    footer: string;
    externalLink: string;
}

const Tile: React.FC<TileProps> = (props) => (
  <Card >
    <CardHead>
      <img src={props.imgSrc} alt="" style={{height: "50px"}}/>
    </CardHead> 
    <CardHeader>{props.header}</CardHeader>
    <CardBody>{props.body}</CardBody>
    <CardFooter>
      {<div><a href={props.externalLink}>{props.footer}</a></div>}
    </CardFooter>
  </Card>
);

export default Tile;