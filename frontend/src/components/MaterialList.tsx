import * as React from 'react';
import {
    Grid,
    GridItem,
    PageSection
} from '@patternfly/react-core';
import Tile from './MaterialTile';

export declare type SpanValueShape = 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12;
export interface MaterialListProps {
    materials: any[];
    spanValue?: SpanValueShape;
    rowSpanValue?: SpanValueShape;
};

const MaterialList: React.FC<MaterialListProps> = (props) => {
    return (
        <PageSection isFilled >
            <Grid gutter="lg" sm={12} md={6} xl={6} xl2={3}>
                {props.materials.map((mat, i) => (
                    <GridItem key={i}
                        span={props.spanValue ? props.spanValue : 3}
                        rowSpan={props.rowSpanValue ? props.rowSpanValue : 2}
                    >
                        <Tile {...mat}></Tile>
                    </GridItem>
                ))}
            </Grid>
        </PageSection>
    );
};

export default MaterialList;