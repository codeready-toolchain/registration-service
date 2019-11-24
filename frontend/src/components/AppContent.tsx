import * as React from 'react';
import { Route, Switch } from 'react-router-dom';
import Home from './Home';
import Provisioner from './Provisioner';
import Dashboard from './Dashboard';

const AppContent: React.FC<{}> = () => {
    return (
        <Switch>
            <Route exact path="/Provision">
                <Provisioner />
            </Route>
            <Route exact path="/Dashboard">
                <Dashboard />
            </Route>
            <Route exact path="/Home">
                <Home />
            </Route>
            <Route exact path="/">
                <div>Loading...</div>
            </Route>
        </Switch>
    );
};

export default AppContent;
