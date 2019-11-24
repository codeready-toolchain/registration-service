import * as React from 'react';
import {Stack, StackItem} from '@patternfly/react-core';
import MaterialList from './MaterialList';
import DashboardData from './DashboardData';
import TrainingData from './TrainingData';
import MarketingData from './MarketingData';

const Dashboard: React.FC<{}> = () => {
    return (
        <Stack>
            <StackItem>
                <MaterialList materials={DashboardData.materials} spanValue={6} rowSpanValue={1}/>
            </StackItem>
            <StackItem>
                <MaterialList materials={TrainingData.materials} />
            </StackItem>
            <StackItem>
                <MaterialList materials={MarketingData.materials} />
            </StackItem>
        </Stack>
    );
};

export default Dashboard;