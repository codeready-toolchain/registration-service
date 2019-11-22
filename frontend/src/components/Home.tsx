import * as React from 'react';
import {Stack, StackItem} from '@patternfly/react-core';
import ProvisionTOS from './ProvisionTOS';
import Marketing from './Marketing';
import MarketingData from './MarketingData';

const Home: React.FC<{}> = () => {
    return (
        <Stack>
            <StackItem>
                <ProvisionTOS />
            </StackItem>
            <StackItem>
                <Marketing materials={MarketingData.materials} />
            </StackItem>
        </Stack>
    );
};

export default Home;