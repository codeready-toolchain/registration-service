import * as React from 'react';
import {
    Grid,
    GridItem,
    PageSection
} from '@patternfly/react-core';
import Tile from './MarketingTile';

export interface MarketingListProps {
    materials: any[];
};

const MarketingList: React.FC<MarketingListProps> = (props) => {
    return (
        <PageSection isFilled >
            <Grid gutter="lg" span={2} sm={12} md={6} xl={6} xl2={3}>
                {props.materials.map((mat, i) => (
                    <GridItem key={i}>
                        <Tile {...mat}></Tile>
                    </GridItem>
                ))}
            </Grid>
        </PageSection>
    );
};

export default MarketingList;