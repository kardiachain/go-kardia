pragma solidity ^0.4.23;

contract A{
    int public data;

    function setData(int _data) public {
        data = _data;
    }
}

contract B{

    int public datab;

    function testData(address aAddr, int _data) public {
        A a = A(aAddr);
        a.setData(_data);
        datab = _data;
    }
}